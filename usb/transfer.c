// Copyright 2013 Google Inc.  All rights reserved.
// Copyright 2016 the gousb Authors.  All rights reserved.
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

#include <libusb-1.0/libusb.h>
#include <stdio.h>
#include <string.h>

void print_xfer(struct libusb_transfer *xfer);
void xfer_callback(void *);

void callback(struct libusb_transfer *xfer) {
	xfer_callback(xfer->user_data);
}

int submit(struct libusb_transfer *xfer) {
	xfer->callback = &callback;
	xfer->status = -1;
	return libusb_submit_transfer(xfer);
}

void print_xfer(struct libusb_transfer *xfer) {
	int i;

	printf("Transfer:\n");
	printf("  dev_handle:   %p\n", xfer->dev_handle);
	printf("  flags:        %08x\n", xfer->flags);
	printf("  endpoint:     %x\n", xfer->endpoint);
	printf("  type:         %x\n", xfer->type);
	printf("  timeout:      %dms\n", xfer->timeout);
	printf("  status:       %x\n", xfer->status);
	printf("  length:       %d (act: %d)\n", xfer->length, xfer->actual_length);
	printf("  callback:     %p\n", xfer->callback);
	printf("  user_data:    %p\n", xfer->user_data);
	printf("  buffer:       %p\n", xfer->buffer);
	printf("  num_iso_pkts: %d\n", xfer->num_iso_packets);
	printf("  packets:\n");
	for (i = 0; i < xfer->num_iso_packets; i++) {
		printf("    [%04d] %d (act: %d) %x\n", i,
			xfer->iso_packet_desc[i].length,
			xfer->iso_packet_desc[i].actual_length,
			xfer->iso_packet_desc[i].status);
	}
}

// extract data from a non-isochronous transfer.
int extract_data(struct libusb_transfer *xfer, void *raw, int max, unsigned char *status) {
	int i;
	unsigned char *in = xfer->buffer;
	unsigned char *out = raw;

    int len = xfer->actual_length;
    if (len > max) {
            len = max;
    }
    memcpy(out, in, len);
   	if (xfer->status != 0 && *status == 0) {
            *status = xfer->status;
   	}
    return len;
}

// extract data from an isochronous transfer. Very similar to extract_data, but
// acquires data from xfer->iso_packet_desc array instead of xfer attributes.
int extract_iso_data(struct libusb_transfer *xfer, void *raw, int max, unsigned char *status) {
	int i;
	int copied = 0;
	unsigned char *in = xfer->buffer;
	unsigned char *out = raw;
	for (i = 0; i < xfer->num_iso_packets; i++) {
		struct libusb_iso_packet_descriptor pkt = xfer->iso_packet_desc[i];

		// Copy the data
		int len = pkt.actual_length;
		if (copied + len > max) {
			len = max - copied;
		}
		memcpy(out, in, len);
		copied += len;

		// Increment offsets
		in += pkt.length;
		out += len;

        if (copied == max) {
                break;
        }

		// Extract first error
		if (pkt.status == 0 || *status != 0) {
			continue;
		}
		*status = pkt.status;
	}
	return copied;
}