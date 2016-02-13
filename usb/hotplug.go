package usb

/*
#include <libusb-1.0/libusb.h>

extern int attachCallback(
	libusb_context* ctx,
	libusb_hotplug_event events,
	libusb_hotplug_flag flags,
	int vid,
	int pid,
	int dev_class,
	void* user_data,
	libusb_hotplug_callback_handle* handle
);
*/
import "C"

import (
	"errors"
	"sync"
	"unsafe"
)

type (
	HotplugEvent  C.int
	hotplugHandle C.libusb_hotplug_callback_handle
	// HotplugOpener is a function that will open the device that was plugged in.
	HotplugOpener func() (*Device, error)
	// A HotplugCallback function is called when a hotplug event is received.
	// The function is called with the event, a descriptor, and a function which,
	// when called, opens the device that generated the hotplug event,
	// if the event was HOTPLUG_EVENT_DEVICE_ARRIVED.
	// Opened devices must be closed in case of a HOTPLUG_EVENT_DEVICE_LEFT event.
	//
	// Returning true from the callback function unregisters the callback.
	HotplugCallback func(desc *Descriptor, event HotplugEvent, opener HotplugOpener) bool
)

type hotplugCallbackData struct {
	f   HotplugCallback // The user's function pointer
	d   interface{}     // The user's userdata
	ctx *Context        // Pointer to the Context struct
	h   hotplugHandle   // the handle belonging to this callback
}

type hotplugCallbackMap struct {
	byHandle map[hotplugHandle]*hotplugCallbackData // lazily initialized in Register
	sync.Mutex
}

const (
	HOTPLUG_EVENT_DEVICE_ARRIVED HotplugEvent = C.LIBUSB_HOTPLUG_EVENT_DEVICE_ARRIVED
	HOTPLUG_EVENT_DEVICE_LEFT    HotplugEvent = C.LIBUSB_HOTPLUG_EVENT_DEVICE_LEFT
	HOTPLUG_ANY                  int          = C.LIBUSB_HOTPLUG_MATCH_ANY
)

//export goCallback
func goCallback(ctx unsafe.Pointer, device unsafe.Pointer, event int, userdata unsafe.Pointer) C.int {
	realCallback := (*hotplugCallbackData)(userdata)
	dev := (*C.libusb_device)(device)
	descriptor, err := newDescriptor(dev)
	if err != nil {
		// TODO: what to do here? add an error callback?
		panic("error happened in callback")
		//return 0
	}
	var opener HotplugOpener
	if HotplugEvent(event) == HOTPLUG_EVENT_DEVICE_ARRIVED {
		opener = func() (*Device, error) {
			var handle *C.libusb_device_handle
			if errno := C.libusb_open(dev, &handle); errno != 0 {
				return nil, usbError(errno)
			}
			return newDevice(handle, descriptor), nil
		}
	} else {
		opener = func() (*Device, error) {
			return nil, errors.New("Opener called when event is not HOTPLUG_EVENT_DEVICE_ARRIVED.")
		}
	}
	if realCallback.f(descriptor, HotplugEvent(event), opener) {
		// let the garbage collector delete the callback
		realCallback.ctx.hotplugCallbacks.Lock()
		defer realCallback.ctx.hotplugCallbacks.Unlock()
		delete(realCallback.ctx.hotplugCallbacks.byHandle, realCallback.h)
		return 1
	} else {
		return 0
	}
}

// RegisterHotplugCallback registers a hotplug callback function. The callback will fire when
// a matching event occurs on a device matching vendorID, productID and class.
// HOTPLUG_ANY can be used to match any vendor ID, product ID or device class.
// It returns a function that can be called to deregister the callback.
// Returning true from the callback will also deregister the callback.
func (ctx *Context) RegisterHotplugCallback(vendorID int, productID int, class int, callback HotplugCallback, events HotplugEvent, enumerate bool) (func(), error) {
	if ctx.hotplugCallbacks.byHandle == nil {
		ctx.hotplugCallbacks.byHandle = make(map[hotplugHandle]*hotplugCallbackData)
	}
	ctx.hotplugCallbacks.Lock()
	defer ctx.hotplugCallbacks.Unlock()
	var handle hotplugHandle
	data := hotplugCallbackData{
		f:   callback,
		ctx: ctx,
	}
	dataPtr := unsafe.Pointer(&data)

	var enumflag C.libusb_hotplug_flag
	if enumerate {
		enumflag = 1
	} else {
		enumflag = 0
	}

	res := C.attachCallback(ctx.ctx, C.libusb_hotplug_event(events), enumflag, C.int(vendorID), C.int(productID), C.int(class), dataPtr, (*C.libusb_hotplug_callback_handle)(&handle))
	if res != C.LIBUSB_SUCCESS {
		return nil, usbError(res)
	}
	data.h = handle
	// protect the data from the garbage collector
	ctx.hotplugCallbacks.byHandle[handle] = &data
	return func() {
		ctx.hotplugCallbacks.Lock()
		defer ctx.hotplugCallbacks.Unlock()
		C.libusb_hotplug_deregister_callback(ctx.ctx, C.libusb_hotplug_callback_handle(handle))
		delete(ctx.hotplugCallbacks.byHandle, handle)
	}, nil
}
