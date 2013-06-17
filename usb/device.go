package usb

// #include <libusb-1.0/libusb.h>
import "C"

import (
	"fmt"
	//"log" // TODO(kevlar): make a logger
	"reflect"
	"sync"
	"time"
	"unsafe"
)

var DefaultReadTimeout = 1 * time.Second
var DefaultWriteTimeout = 1 * time.Second
var DefaultControlTimeout = 250 * time.Millisecond //5 * time.Second

type Device struct {
	handle *C.libusb_device_handle

	// Embed the device information for easy access
	*Descriptor

	// Timeouts
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	ControlTimeout time.Duration

	// Claimed interfaces
	lock    *sync.Mutex
	claimed map[uint8]int
}

func newDevice(handle *C.libusb_device_handle, desc *Descriptor) *Device {
	ifaces := 0
	d := &Device{
		handle:         handle,
		Descriptor:     desc,
		ReadTimeout:    DefaultReadTimeout,
		WriteTimeout:   DefaultWriteTimeout,
		ControlTimeout: DefaultControlTimeout,
		lock:           new(sync.Mutex),
		claimed:        make(map[uint8]int, ifaces),
	}

	return d
}

func (d *Device) Reset() error {
	if errno := C.libusb_reset_device(d.handle); errno != 0 {
		return usbError(errno)
	}
	return nil
}

func (d *Device) Control(rType, request uint8, val, idx uint16, data []byte) (int, error) {
	//log.Printf("control xfer: %d:%d/%d:%d %x", idx, rType, request, val, string(data))
	dataSlice := (*reflect.SliceHeader)(unsafe.Pointer(&data))
	n := C.libusb_control_transfer(
		d.handle,
		C.uint8_t(rType),
		C.uint8_t(request),
		C.uint16_t(val),
		C.uint16_t(idx),
		(*C.uchar)(unsafe.Pointer(dataSlice.Data)),
		C.uint16_t(len(data)),
		C.uint(d.ControlTimeout/time.Millisecond))
	if n < 0 {
		return int(n), usbError(n)
	}
	return int(n), nil
}

// ActiveConfig returns the config id (not the index) of the active configuration.
// This corresponds to the ConfigInfo.Config field.
func (d *Device) ActiveConfig() (uint8, error) {
	var cfg C.int
	if errno := C.libusb_get_configuration(d.handle, &cfg); errno < 0 {
		return 0, usbError(errno)
	}
	return uint8(cfg), nil
}

// SetConfig attempts to change the active configuration.
// The cfg provided is the config id (not the index) of the configuration to set,
// which corresponds to the ConfigInfo.Config field.
func (d *Device) SetConfig(cfg uint8) error {
	if errno := C.libusb_set_configuration(d.handle, C.int(cfg)); errno < 0 {
		return usbError(errno)
	}
	return nil
}

// Close the device.
func (d *Device) Close() error {
	if d.handle == nil {
		return fmt.Errorf("usb: double close on device")
	}
	d.lock.Lock()
	defer d.lock.Unlock()
	for iface := range d.claimed {
		C.libusb_release_interface(d.handle, C.int(iface))
	}
	C.libusb_close(d.handle)
	d.handle = nil
	return nil
}

func (d *Device) OpenEndpoint(conf, iface, setup, epoint uint8) (Endpoint, error) {
	end := &endpoint{
		Device: d,
	}

	for _, c := range d.Configs {
		if c.Config != conf {
			continue
		}
		fmt.Printf("found conf: %#v\n", c)
		for _, i := range c.Interfaces {
			if i.Number != iface {
				continue
			}
			fmt.Printf("found iface: %#v\n", i)
			for _, s := range i.Setups {
				if s.Alternate != setup {
					continue
				}
				fmt.Printf("found setup: %#v\n", s)
				for _, e := range s.Endpoints {
					fmt.Printf("ep %02x search: %#v\n", epoint, s)
					if e.Address != epoint {
						continue
					}
					end.InterfaceSetup = s
					end.EndpointInfo = e
					switch tt := TransferType(e.Attributes) & TRANSFER_TYPE_MASK; tt {
					case TRANSFER_TYPE_BULK:
						end.xfer = bulk_xfer
					case TRANSFER_TYPE_INTERRUPT:
						end.xfer = interrupt_xfer
					case TRANSFER_TYPE_ISOCHRONOUS:
						end.xfer = isochronous_xfer
					default:
						return nil, fmt.Errorf("usb: %s transfer is unsupported", tt)
					}
					goto found
				}
				return nil, fmt.Errorf("usb: unknown endpoint %02x", epoint)
			}
			return nil, fmt.Errorf("usb: unknown setup %02x", setup)
		}
		return nil, fmt.Errorf("usb: unknown interface %02x", iface)
	}
	return nil, fmt.Errorf("usb: unknown configuration %02x", conf)

found:

	// Set the configuration
	var activeConf C.int
	if errno := C.libusb_get_configuration(d.handle, &activeConf); errno < 0 {
		return nil, fmt.Errorf("usb: getcfg: %s", usbError(errno))
	}
	if int(activeConf) != int(conf) {
		if errno := C.libusb_set_configuration(d.handle, C.int(conf)); errno < 0 {
			return nil, fmt.Errorf("usb: setcfg: %s", usbError(errno))
		}
	}

	// Claim the interface
	if errno := C.libusb_claim_interface(d.handle, C.int(iface)); errno < 0 {
		return nil, fmt.Errorf("usb: claim: %s", usbError(errno))
	}

	// Increment the claim count
	d.lock.Lock()
	d.claimed[iface]++
	d.lock.Unlock() // unlock immediately because the next calls may block

	// Choose the alternate
	// This doesn't seem to work...
	if errno := C.libusb_set_interface_alt_setting(d.handle, C.int(iface), C.int(setup)); errno < 0 {
		//log.Printf("ignoring altsetting error: %s", usbError(errno))
		return nil, fmt.Errorf("usb: setalt: %s", usbError(errno))
	}

	return end, nil
}
