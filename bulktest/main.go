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
	"time"
	"log"
	"strconv"

	"github.com/JohnFarmer/gousb/usb"
	//"github.com/kylelemons/gousb/usbid"
)

var (
	device_vid_pid   = flag.String(
		"device",
		"0483:5741",
		"Device to which to connect")
	config   = flag.Int("config", 1, "Endpoint to which to connect")
	iface    = flag.Int("interface", 0, "Endpoint to which to connect")
	setup    = flag.Int("setup", 0, "Endpoint to which to connect")
	endpoint_in = flag.Int("endpoint_in", 1, "Endpoint to which to connect")
	endpoint_out = flag.Int("endpoint_out", 3, "Endpoint to which to connect")
	debug    = flag.Int("debug", 3, "Debug level for libusb")
	
	count = int(0)
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

	log.Printf("Connecting to endpoint_in...")
	log.Printf("- %#v", dev.Descriptor)
	ep_bulk_in, err := dev.OpenEndpoint(uint8(*config), uint8(*iface), uint8(*setup), uint8(*endpoint_in)|uint8(usb.ENDPOINT_DIR_IN))
	if err != nil {
		log.Fatalf("IN-open: %s", err)
	}

	ep_bulk_out, err := dev.OpenEndpoint(uint8(*config), uint8(*iface), uint8(*setup), uint8(*endpoint_out)|uint8(usb.ENDPOINT_DIR_OUT))
	if err != nil {
		log.Fatalf("OUT-open: %s", err)
	}

	// Loop Test of USB Write/Read
	for {
		fmt.Println("-------------------------------------")
		count += 1
		buf_out := make([]byte, 64)
		fmt.Println(count)
		
		buf_out = []byte(strconv.Itoa(count))
		ep_bulk_out.Write(buf_out)

		time.Sleep(1000 * time.Millisecond)

		// make a buffer according to the packet size of endpoint_in
		// should be a multiple of <packet size> like 64/128/256
		buf_in := make([]byte, 64)
		ep_bulk_in.Read(buf_in)

		fmt.Println(string(buf_in[:]))
		fmt.Printf("%c\n", buf_in)
	}
	
	// above code read from endpoints shown below 
	// (the device is a stm32 board with USB program,
	// program on the board would be found on Github later
	// there is still a little bug right now)
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
	/*
      Endpoint Descriptor:
             bLength                 7
             bDescriptorType         5
             bEndpointAddress     0x03  EP 3 OUT
             bmAttributes            2
               Transfer Type            Bulk
               Synch Type               None
               Usage Type               Data
             wMaxPacketSize     0x0040  1x 64 bytes
             bInterval               0                */
}
