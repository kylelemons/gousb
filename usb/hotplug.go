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
	"unsafe"
	"sync"
)

type (
	HotplugEvent C.int
	HotplugHandle C.libusb_hotplug_callback_handle
	HotplugCallback func(desc *Descriptor, event HotplugEvent) bool
)

type hotplugCallbackData struct {
   f HotplugCallback  // The user's function pointer
   d interface{}      // The user's userdata.
}

const (
	HOTPLUG_EVENT_DEVICE_ARRIVED HotplugEvent = C.LIBUSB_HOTPLUG_EVENT_DEVICE_ARRIVED
	HOTPLUG_EVENT_DEVICE_LEFT    HotplugEvent = C.LIBUSB_HOTPLUG_EVENT_DEVICE_LEFT
	HOTPLUG_ANY                  int = C.LIBUSB_HOTPLUG_MATCH_ANY
)

var (
	hotplugCallbackMap      map[HotplugHandle]*hotplugCallbackData
	mutexHotplugCallbackMap sync.Mutex
)

func init() {
	hotplugCallbackMap = make(map[HotplugHandle]*hotplugCallbackData)
}

//export goCallback
func goCallback(ctx unsafe.Pointer, device unsafe.Pointer, event int, userdata unsafe.Pointer) C.int {
	realCallback := (*hotplugCallbackData)(userdata)
	descriptor, err := newDescriptor((*C.libusb_device)(device))
	if err != nil {
		// TODO: what to do here? add an error callback?
		panic("error happened in callback")
		//return 0
	}
	if realCallback.f(descriptor, HotplugEvent(event)) {
		return 1
	} else {
		return 0
	}
}

func (ctx *Context) RegisterHotplugCallback(vendor_id int, product_id int, class int, callback HotplugCallback, events HotplugEvent, enumerate bool) (HotplugHandle, error) {
	mutexHotplugCallbackMap.Lock()
	defer mutexHotplugCallbackMap.Unlock()
	var handle HotplugHandle
	data := hotplugCallbackData{
		f: callback,
	}
	dataPtr := unsafe.Pointer(&data)

	var enumflag C.libusb_hotplug_flag
	if enumerate {
		enumflag = 1
	} else {
		enumflag = 0
	}

	res := C.attachCallback(ctx.ctx, C.libusb_hotplug_event(events), enumflag, C.int(vendor_id), C.int(product_id), C.int(class), dataPtr, (*C.libusb_hotplug_callback_handle)(&handle))
	if res != C.LIBUSB_SUCCESS {
		return 0, usbError(res)
	}
	// protect the data from the garbage collector
	hotplugCallbackMap[handle] = &data
	return handle, nil
}

func (ctx *Context) DeregisterHotplugCallback(handle HotplugHandle) {
	mutexHotplugCallbackMap.Lock()
	defer mutexHotplugCallbackMap.Unlock()
	C.libusb_hotplug_deregister_callback(ctx.ctx, C.libusb_hotplug_callback_handle(handle))
	delete(hotplugCallbackMap, handle)
}
