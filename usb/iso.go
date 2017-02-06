// Copyright 2013 Google Inc.  All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package usb

/*
#include <libusb-1.0/libusb.h>

int submit(struct libusb_transfer *xfer);
void print_xfer(struct libusb_transfer *xfer);
int extract_data(struct libusb_transfer *xfer, void *data, int max, unsigned char *status);
*/
import "C"

import (
	"fmt"
	"log"
	"time"
	"unsafe"
)

//export iso_callback
func iso_callback(cptr unsafe.Pointer) {
	ch := *(*chan struct{})(cptr)
	close(ch)
}

func (end *endpoint) allocTransfer() *Transfer {
	const (
		iso_packets = 200
	)

	xfer := C.libusb_alloc_transfer(C.int(iso_packets))
	if xfer == nil {
		log.Printf("usb: transfer allocation failed?!")
		return nil
	}

	buf := make([]byte, iso_packets*end.EndpointInfo.MaxIsoPacket)
	done := make(chan struct{}, 1)

	xfer.dev_handle = end.Device.handle
	xfer.endpoint = C.uchar(end.Address)
	xfer._type = C.LIBUSB_TRANSFER_TYPE_ISOCHRONOUS

	xfer.buffer = (*C.uchar)((unsafe.Pointer)(&buf[0]))
	xfer.length = C.int(len(buf))
	xfer.num_iso_packets = iso_packets

	C.libusb_set_iso_packet_lengths(xfer, C.uint(end.EndpointInfo.MaxIsoPacket))
	/*
		pkts := *(*[]C.struct_libusb_packet_descriptor)(unsafe.Pointer(&reflect.SliceHeader{
			Data: uintptr(unsafe.Pointer(&xfer.iso_packet_desc)),
			Len:  iso_packets,
			Cap:  iso_packets,
		}))
	*/

	t := &Transfer{
		xfer: xfer,
		done: done,
		buf:  buf,
	}
	xfer.user_data = (unsafe.Pointer)(&t.done)

	return t
}

type Transfer struct {
	xfer *C.struct_libusb_transfer
	pkts []*C.struct_libusb_packet_descriptor
	done chan struct{}
	buf  []byte
}

func (t *Transfer) Submit(timeout time.Duration) error {
	//log.Printf("iso: submitting %#v", t.xfer)
	t.xfer.timeout = C.uint(timeout / time.Millisecond)
	if errno := C.submit(t.xfer); errno < 0 {
		return usbError(errno)
	}
	return nil
}

func (t *Transfer) Wait(b []byte) (n int, err error) {
	select {
	case <-time.After(10 * time.Second):
		return 0, fmt.Errorf("wait timed out after 10s")
	case <-t.done:
	}
	// Non-iso transfers:
	//n = int(t.xfer.actual_length)
	//copy(b, ((*[1 << 16]byte)(unsafe.Pointer(t.xfer.buffer)))[:n])

	//C.print_xfer(t.xfer)
	/*
		buf, offset := ((*[1 << 16]byte)(unsafe.Pointer(t.xfer.buffer))), 0
		for i, pkt := range *t.pkts {
			log.Printf("Type is %T", t.pkts)
			n += copy(b[n:], buf[offset:][:pkt.actual_length])
			offset += pkt.Length
			if pkt.status != 0 && err == nil {
				err = error(TransferStatus(pkt.status))
			}
		}
	*/
	var status uint8
	n = int(C.extract_data(t.xfer, unsafe.Pointer(&b[0]), C.int(len(b)), (*C.uchar)(unsafe.Pointer(&status))))
	if status != 0 {
		err = TransferStatus(status)
	}
	return n, err
}

func (t *Transfer) Close() error {
	C.libusb_free_transfer(t.xfer)
	return nil
}

// a handler for isochronous transfer endpoints.
func isochronous_xfer(e *endpoint, buf []byte, timeout time.Duration) (int, error) {
	if EndpointDirection(e.Address)&ENDPOINT_DIR_MASK != ENDPOINT_DIR_IN {
		return 0, fmt.Errorf("usb: write: gousb supports only IN isochronous transfers")
	}
	if e.isoXfers == nil {
		e.isoXfers = &isoXfers{}
		e.isoXfers.init(timeout)
		for i := 0; i < isoXfersInFlight; i++ {
			e.isoXfers.transfers <- e.allocTransfer()
		}

	}
	// refill the in-flight queue
	go func() { e.isoXfers.submit(e.allocTransfer()) }()
	return e.isoXfers.get(buf)
}

// a (potentially) submitted transfer
type xferStatus struct {
	t *Transfer
	// err holds the result of t.Submit
	err error
}

// represents a queue of iso transfers scheduled. It keeps multiple requests in flight to ensure that
// anytime the host receives an transfer response, it has another transfer request ready to send.
type isoXfers struct {
	// unsubmitted transfers
	transfers chan *Transfer
	// submitted transfers
	submitStatus chan xferStatus
}

const isoXfersInFlight = 5

// submitter runs in the background, ensuring that there are always transfers ready for submission.
func (x *isoXfers) submitter(timeout time.Duration) {
	for t := range x.transfers {
		err := t.Submit(timeout)
		x.submitStatus <- xferStatus{t, err}
	}
}

// init initializes an empty isoXfers and starts the submitter.
func (x *isoXfers) init(timeout time.Duration) {
	x.transfers = make(chan *Transfer, isoXfersInFlight)
	x.submitStatus = make(chan xferStatus, isoXfersInFlight)
	go x.submitter(timeout)
}

// stop stops the submitted
func (x *isoXfers) stop() {
	close(x.transfers)
	for i := 0; i < isoXfersInFlight; i++ {
		<-x.submitStatus
	}
}

// submit sends a new transfer request.
func (x *isoXfers) submit(t *Transfer) {
	x.transfers <- t
}

// get retrieves a transfer from the queue.
func (x *isoXfers) get(buf []byte) (int, error) {
	st := <-x.submitStatus
	defer st.t.Close()
	if st.err != nil {
		log.Printf("iso: xfer failed to submit: %s", st.err)
		return 0, st.err
	}
	num, err := st.t.Wait(buf)
	if err != nil {
		log.Printf("iso: xfer failed: %s", err)
		return 0, err
	}
	return num, nil
}
