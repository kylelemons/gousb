package usb

/*
#include <libusb-1.0/libusb.h>
#include <stdlib.h>

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
	"fmt"
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
	ctx *Context        // Pointer to the Context struct
}

type hotplugCallbackMap struct {
	byID map[unsafe.Pointer]*hotplugCallbackData // lazily initialized in Register
	sync.Mutex
}

const (
	HOTPLUG_EVENT_DEVICE_ARRIVED HotplugEvent = C.LIBUSB_HOTPLUG_EVENT_DEVICE_ARRIVED
	HOTPLUG_EVENT_DEVICE_LEFT    HotplugEvent = C.LIBUSB_HOTPLUG_EVENT_DEVICE_LEFT
	HOTPLUG_ANY                  int          = C.LIBUSB_HOTPLUG_MATCH_ANY
)

var (
	callbacks hotplugCallbackMap
)

// add adds the data to the map and returns its id
func (m *hotplugCallbackMap) add(data *hotplugCallbackData) unsafe.Pointer {
	m.Lock()
	defer m.Unlock()

	if m.byID == nil {
		m.byID = make(map[unsafe.Pointer]*hotplugCallbackData)
	}

	// allocate memory to pass to C as user_data,
	// but instead of using it to store the map key,
	// just use the pointer as the map key.
	id := C.malloc(1)
	if id == nil {
		panic(errors.New("Failed to allocate memory during callback registration"))
	}
	m.byID[id] = data
	return id
}

func (m *hotplugCallbackMap) get(id unsafe.Pointer) (data *hotplugCallbackData, ok bool) {
	data, ok = m.byID[id]
	return
}

func (m *hotplugCallbackMap) remove(id unsafe.Pointer) {
	m.Lock()
	defer m.Unlock()
	_, ok := m.byID[id]
	if ok {
		delete(m.byID, id)
		C.free(id)
	}
}

func (c *Context) addCallback(id unsafe.Pointer) {
	c.hotplugCallbackMutex.Lock()
	defer c.hotplugCallbackMutex.Unlock()
	if c.hotplugCallbacks == nil {
		c.hotplugCallbacks = make(map[unsafe.Pointer]empty)
	}
	c.hotplugCallbacks[id] = empty{}
}

func (c *Context) removeCallback(id unsafe.Pointer) {
	c.hotplugCallbackMutex.Lock()
	defer c.hotplugCallbackMutex.Unlock()
	delete(c.hotplugCallbacks, id)
}

// cleanupCallbacks is called on Close, and it removes the callbacks
// belonging to the context in the callbacks map
func (c *Context) cleanupCallbacks() {
	c.hotplugCallbackMutex.Lock()
	defer c.hotplugCallbackMutex.Unlock()
	for id := range c.hotplugCallbacks {
		callbacks.remove(id)
	}
}

//export goCallback
func goCallback(ctx unsafe.Pointer, device unsafe.Pointer, event int, userdata unsafe.Pointer) C.int {
	dataID := userdata
	realCallback, ok := callbacks.get(dataID)
	if !ok {
		panic(fmt.Errorf("USB hotplug callback function not found"))
	}
	dev := (*C.libusb_device)(device)
	descriptor, err := newDescriptor(dev)
	if err != nil {
		// TODO: what to do here? add an error callback?
		panic(fmt.Errorf("Error during USB hotplug callback: %s", err))
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
		// callback will be unregistered, delete the data from memory.
		callbacks.remove(dataID)
		realCallback.ctx.removeCallback(dataID)
		return 1
	} else {
		return 0
	}
}

// RegisterHotplugCallback registers a hotplug callback function. The callback will fire when
// a matching event occurs on a device matching vendorID, productID and class.
// HOTPLUG_ANY can be used to match any vendor ID, product ID or device class.
// If enumerate is true, the callback is called for matching currently attached devices.
// It returns a function that can be called to deregister the callback.
// Returning true from the callback will also deregister the callback.
// Closing the context will deregister all callbacks automatically.
func (c *Context) RegisterHotplugCallback(vendorID int, productID int, class int, enumerate bool, callback HotplugCallback, event HotplugEvent, events ...HotplugEvent) (func(), error) {
	var handle hotplugHandle
	data := hotplugCallbackData{
		f:   callback,
		ctx: c,
	}
	// store the data in go memory, since we can't pass it to cgo.
	dataID := callbacks.add(&data)
	c.addCallback(dataID)

	var enumflag C.libusb_hotplug_flag
	if enumerate {
		enumflag = 1
	} else {
		enumflag = 0
	}

	for _, e := range events {
		event |= e
	}

	res := C.attachCallback(c.ctx, C.libusb_hotplug_event(event), enumflag, C.int(vendorID), C.int(productID), C.int(class), dataID, (*C.libusb_hotplug_callback_handle)(&handle))
	if res != C.LIBUSB_SUCCESS {
		callbacks.remove(dataID)
		c.removeCallback(dataID)
		return nil, usbError(res)
	}
	return func() {
		C.libusb_hotplug_deregister_callback(c.ctx, C.libusb_hotplug_callback_handle(handle))
		callbacks.remove(dataID)
		c.removeCallback(dataID)
	}, nil
}
