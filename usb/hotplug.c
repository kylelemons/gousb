#include <libusb-1.0/libusb.h>
#include "_cgo_export.h"

int attachCallback(
	libusb_context* ctx,
	libusb_hotplug_event events,
	libusb_hotplug_flag flags,
	int vid,
	int pid,
	int dev_class,
	void* user_data,
	libusb_hotplug_callback_handle* handle
) {
	return libusb_hotplug_register_callback(
		ctx, events, flags, vid, pid, dev_class, (libusb_hotplug_callback_fn)(goCallback), user_data, handle
	);
}
