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

// rawread attempts to read from the specified USB device.
package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/JohnFarmer/gousb/usb"
	"github.com/JohnFarmer/gousb/usbid"
)

var (
	device_vid_pid   = flag.String(
		"device",
		"0483:5741",
		"Device to which to connect")
	config   = flag.Int("config", 1, "Endpoint to which to connect")
	iface    = flag.Int("interface", 0, "Endpoint to which to connect")
	setup    = flag.Int("setup", 0, "Endpoint to which to connect")
	endpoint = flag.Int("endpoint", 1, "Endpoint to which to connect")
	debug    = flag.Int("debug", 3, "Debug level for libusb")
)

func main() {
	flag.Parse()

	// Only one context should be needed for an application.
	// It should always be closed.
	ctx := usb.NewContext()
	defer ctx.Close()
	ctx.Debug(*debug)

	log.Printf("Scanning for device %q...", *device_vid_pid)

	// Get device by VID:PID string like "xxxx:xxxx"
	dev, _ := ctx.GetDeviceWithVidPid(*device_vid_pid)
	defer dev.Close()

	log.Printf("Connecting to endpoint...")
	log.Printf("- %#v", dev.Descriptor)
	ep, err := dev.OpenEndpoint(uint8(*config), uint8(*iface), uint8(*setup), uint8(*endpoint)|uint8(usb.ENDPOINT_DIR_IN))
	if err != nil {
		log.Fatalf("open: %s", err)
	}


	// make a buffer according to the packet size of endpoint
	// should be a multiple of <packet size> like 64/128/256
	buf := make([]byte, 64)
	ep.Read(buf)

	fmt.Println(string(buf[:]))
	fmt.Printf("%c\n", buf)
	
	// above code read from a end point like this
	// (the device is a stm32 board with USB program,
	// program on the board can be found on Github later)
	/*
      Endpoint Descriptor:
              bLength                 7
              bDescriptorType         5
              bEndpointAddress     0x81  EP 1 IN
              bmAttributes            2
                Transfer Type            Bulk
                Synch Type               None
                Usage Type               Data
              wMaxPacketSize     0x0040  1x 64 bytes
              bInterval               0               */
}
