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

// #include <libusb-1.0/libusb.h>
import "C"

import (
	"fmt"
	"reflect"
	"time"
	"unsafe"
)

// Endpoint represents a USB interface endpoint.
type Endpoint interface {
	// Read requests an IN transfer from the endpoint.
	Read(b []byte) (int, error)
	// Write sends an OUT transfer to the endpoint.
	Write(b []byte) (int, error)
	// Interface returns the InterfaceSetup of the interface to which this endpoint belongs.
	Interface() InterfaceSetup
	// Info returns the EndpointInfo of this endpoint.
	Info() EndpointInfo
	// Close releases the resources associated with the endpoint.
	Close()
}

type endpoint struct {
	*Device
	InterfaceSetup
	EndpointInfo
	xfer     func(*endpoint, []byte, time.Duration) (int, error)
	isoXfers *isoXfers
}

// Read implements Endpoint.
func (e *endpoint) Read(buf []byte) (int, error) {
	if EndpointDirection(e.Address)&ENDPOINT_DIR_MASK != ENDPOINT_DIR_IN {
		return 0, fmt.Errorf("usb: read: not an IN endpoint")
	}

	return e.xfer(e, buf, e.ReadTimeout)
}

// Write implements Endpoint.
func (e *endpoint) Write(buf []byte) (int, error) {
	if EndpointDirection(e.Address)&ENDPOINT_DIR_MASK != ENDPOINT_DIR_OUT {
		return 0, fmt.Errorf("usb: write: not an OUT endpoint")
	}

	return e.xfer(e, buf, e.WriteTimeout)
}

// Interface implements Endpoint.
func (e *endpoint) Interface() InterfaceSetup { return e.InterfaceSetup }

// Info implements Endpoint.
func (e *endpoint) Info() EndpointInfo { return e.EndpointInfo }

// Close implements Endpoint.
func (e *endpoint) Close() {
	if e.isoXfers != nil {
		e.isoXfers.stop()
		e.isoXfers = nil
	}
	e.Device.lock.Lock()
	defer e.Device.lock.Unlock()
	if _, ok := e.Device.claimed[e.InterfaceSetup.Number]; ok {
		C.libusb_release_interface(e.Device.handle, C.int(e.InterfaceSetup.Number))
	}
}

// a handler for bulk transfer endpoints.
func bulk_xfer(e *endpoint, buf []byte, timeout time.Duration) (int, error) {
	if len(buf) == 0 {
		return 0, nil
	}

	data := (*reflect.SliceHeader)(unsafe.Pointer(&buf)).Data

	var cnt C.int
	if errno := C.libusb_bulk_transfer(
		e.handle,
		C.uchar(e.Address),
		(*C.uchar)(unsafe.Pointer(data)),
		C.int(len(buf)),
		&cnt,
		C.uint(timeout/time.Millisecond)); errno < 0 {
		return 0, usbError(errno)
	}
	return int(cnt), nil
}

// a handler for interrupt transfer endpoints.
func interrupt_xfer(e *endpoint, buf []byte, timeout time.Duration) (int, error) {
	if len(buf) == 0 {
		return 0, nil
	}

	data := (*reflect.SliceHeader)(unsafe.Pointer(&buf)).Data

	var cnt C.int
	if errno := C.libusb_interrupt_transfer(
		e.handle,
		C.uchar(e.Address),
		(*C.uchar)(unsafe.Pointer(data)),
		C.int(len(buf)),
		&cnt,
		C.uint(timeout/time.Millisecond)); errno < 0 {
		return 0, usbError(errno)
	}
	return int(cnt), nil
}
