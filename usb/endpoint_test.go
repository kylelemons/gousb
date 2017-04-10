// Copyright 2017 the gousb Authors.  All rights reserved.
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

import (
	"reflect"
	"testing"
	"time"
)

func TestEndpoint(t *testing.T) {
	defer func(i libusbIntf) { libusb = i }(libusb)
	for _, epCfg := range []struct {
		method string
		InterfaceSetup
		EndpointInfo
	}{
		{"Read", testBulkInSetup, testBulkInEP},
		{"Write", testIsoOutSetup, testIsoOutEP},
	} {
		t.Run(epCfg.method, func(t *testing.T) {
			for _, tc := range []struct {
				desc    string
				buf     []byte
				ret     int
				status  TransferStatus
				want    int
				wantErr bool
			}{
				{
					desc: "empty buffer",
					buf:  nil,
					ret:  10,
					want: 0,
				},
				{
					desc: "128B buffer, 60 transferred",
					buf:  make([]byte, 128),
					ret:  60,
					want: 60,
				},
				{
					desc:    "128B buffer, 10 transferred and then error",
					buf:     make([]byte, 128),
					ret:     10,
					status:  LIBUSB_TRANSFER_ERROR,
					want:    10,
					wantErr: true,
				},
			} {
				lib := newFakeLibusb()
				libusb = lib
				ep := newEndpoint(nil, epCfg.InterfaceSetup, epCfg.EndpointInfo, time.Second, time.Second)
				op, ok := reflect.TypeOf(ep).MethodByName(epCfg.method)
				if !ok {
					t.Fatalf("method %s not found in endpoint struct", epCfg.method)
				}
				go func() {
					fakeT := lib.waitForSubmitted()
					fakeT.length = tc.ret
					fakeT.status = tc.status
					close(fakeT.done)
				}()
				opv := op.Func.Interface().(func(*endpoint, []byte) (int, error))
				got, err := opv(ep, tc.buf)
				if (err != nil) != tc.wantErr {
					t.Errorf("%s: bulkInEP.Read(): got err: %v, err != nil is %v, want %v", tc.desc, err, err != nil, tc.wantErr)
					continue
				}
				if got != tc.want {
					t.Errorf("%s: bulkInEP.Read(): got %d bytes, want %d", tc.desc, got, tc.want)
				}
			}
		})
	}
}

func TestEndpointWrongDirection(t *testing.T) {
	ep := &endpoint{
		InterfaceSetup: testBulkInSetup,
		EndpointInfo:   testBulkInEP,
	}
	_, err := ep.Write([]byte{1, 2, 3})
	if err == nil {
		t.Error("bulkInEP.Write(): got nil error, want non-nil")
	}
	ep = &endpoint{
		InterfaceSetup: testIsoOutSetup,
		EndpointInfo:   testIsoOutEP,
	}
	_, err = ep.Read(make([]byte, 64))
	if err == nil {
		t.Error("isoOutEP.Read(): got nil error, want non-nil")
	}
}

func TestOpenEndpoint(t *testing.T) {
	origLib := libusb
	defer func() { libusb = origLib }()
	libusb = newFakeLibusb()

	c := NewContext()
	dev, err := c.OpenDeviceWithVidPid(0x8888, 0x0002)
	if dev == nil {
		t.Fatal("OpenDeviceWithVidPid(0x8888, 0x0002): got nil device, need non-nil")
	}
	defer dev.Close()
	if err != nil {
		t.Fatalf("OpenDeviceWithVidPid(0x8888, 0x0002): got error %v, want nil", err)
	}
	ep, err := dev.OpenEndpoint(1, 1, 1, 0x86)
	if err != nil {
		t.Errorf("OpenEndpoint(cfg=1, if=1, alt=1, ep=0x86): got error %v, want nil", err)
	}
	i := ep.Info()
	if got, want := i.Address, uint8(0x86); got != want {
		t.Errorf("OpenEndpoint(cfg=1, if=1, alt=1, ep=0x86): ep.Info.Address = %x, want %x", got, want)
	}
	if got, want := i.MaxIsoPacket, uint32(2*1024); got != want {
		t.Errorf("OpenEndpoint(cfg=1, if=1, alt=1, ep=0x86): ep.Info.MaxIsoPacket = %d, want %d", got, want)
	}
}
