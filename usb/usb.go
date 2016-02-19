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

// Package usb provides a wrapper around libusb-1.0.
package usb

// #cgo LDFLAGS: -lusb-1.0
// #include <libusb-1.0/libusb.h>
import "C"

import (
	"errors"
	"log"
	"reflect"
	"strconv"
	"strings"
	"unsafe"
)

type Context struct {
	ctx  *C.libusb_context
	done chan struct{}
}

func (c *Context) Debug(level int) {
	C.libusb_set_debug(c.ctx, C.int(level))
}

func NewContext() *Context {
	c := &Context{
		done: make(chan struct{}),
	}

	if errno := C.libusb_init(&c.ctx); errno != 0 {
		panic(usbError(errno))
	}

	go func() {
		tv := C.struct_timeval{
			tv_sec:  0,
			tv_usec: 100000,
		}
		for {
			select {
			case <-c.done:
				return
			default:
			}
			if errno := C.libusb_handle_events_timeout_completed(c.ctx, &tv, nil); errno < 0 {
				log.Printf("handle_events: error: %s", usbError(errno))
				continue
			}
			//log.Printf("handle_events returned")
		}
	}()

	return c
}

// ListDevices calls each with each enumerated device.
// If the function returns true, the device is opened and a Device is returned if the operation succeeds.
// Every Device returned (whether an error is also returned or not) must be closed.
// If there are any errors enumerating the devices,
// the final one is returned along with any successfully opened devices.
func (c *Context) ListDevices(each func(desc *Descriptor) bool) ([]*Device, error) {
	var list **C.libusb_device
	cnt := C.libusb_get_device_list(c.ctx, &list)
	if cnt < 0 {
		return nil, usbError(cnt)
	}
	defer C.libusb_free_device_list(list, 1)

	var slice []*C.libusb_device
	*(*reflect.SliceHeader)(unsafe.Pointer(&slice)) = reflect.SliceHeader{
		Data: uintptr(unsafe.Pointer(list)),
		Len:  int(cnt),
		Cap:  int(cnt),
	}

	var reterr error
	ret := []*Device{}
	for _, dev := range slice {
		desc, err := newDescriptor(dev)
		if err != nil {
			reterr = err
			continue
		}

		if each(desc) {
			var handle *C.libusb_device_handle
			if errno := C.libusb_open(dev, &handle); errno != 0 {
				reterr = err
				continue
			}
			ret = append(ret, newDevice(handle, desc))
		}
	}
	return ret, reterr
}

// Open Device with VID:PID string
// NOTE: This function is JUST for QUICK TESTING, and should NOT be used in
// a real application.
// Visit the below URL for detail.
// http://libusb.sourceforge.net/api-1.0/group__dev.html#ga11ba48adb896b1492bbd3d0bf7e0f665
func (c *Context) OpenDeviceWithVidPid(vidpid string) (*Device, error) {
	var hDev *C.libusb_device_handle

	vid_pid := strings.Split(vidpid, ":")
	if len(vid_pid) != 2 {
		return nil, errors.New("invalid VID:PID, " + vidpid)
	}

	vid, err := strconv.ParseInt(vid_pid[0], 16, 16)
	if err != nil {
		return nil, errors.New("invalid VID " + vid_pid[0] + ", " + err.Error())
	}

	pid, err := strconv.ParseInt(vid_pid[1], 16, 16)
	if err != nil {
		return nil, errors.New("invalid PID " + vid_pid[1] + ", " + err.Error())
	}

	hDev = C.libusb_open_device_with_vid_pid(
		c.ctx,
		C.uint16_t(vid),
		C.uint16_t(pid))
	if hDev == nil {
		return nil, errors.New("failed to open device with VID:PID " + vidpid)
	}

	c_dev := C.libusb_get_device(hDev)
	desc, err := newDescriptor(c_dev)
	if err != nil {
		return nil, err
	}

	log.Println("----------------Testing-OpenDeviceWithVidPid----------------")
	log.Printf("%03d.%03d %s:%s\n", desc.Bus, desc.Address, desc.Vendor, desc.Product)
	log.Println("------------------------End-of-Test-------------------------")

	dev := newDevice(hDev, desc)
	return dev, nil
}

func (c *Context) Close() error {
	close(c.done)
	if c.ctx != nil {
		C.libusb_exit(c.ctx)
	}
	c.ctx = nil
	return nil
}
