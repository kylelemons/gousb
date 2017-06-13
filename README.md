Deprecated
==========

This package was deprecated in favor of https://github.com/google/gousb.

This package will still receive bugfixes for reported issues,
 but it will no longer be expanded with new functionality.

Note that the [new package](https://github.com/google/gousb) is not a drop-in
replacement, as some elements of the API have changed. The most important changes:

* device configurations and interfaces now need to be explicitly claimed before
  an endpoint can be used.
* InEndpoint and OutEndpoint are distinct types, with In supporting only read
  operations and Out supporting only write operations.
* InEndpoint has a Stream functionality that allows for a buffered high-bandwidth
  data transfer.
* all the details of USB representation are hidden behind meaningful types.
  It should no longer be necessary to use bitmasks for anything.
* ListDevices is renamed to OpenDevices.

Introduction
============

[![Build Status][ciimg]][ci]
[![GoDoc][docimg]][doc]
[![Coverage Status](https://coveralls.io/repos/github/kylelemons/gousb/badge.svg?branch=master)](https://coveralls.io/github/kylelemons/gousb?branch=master)

The gousb package is an attempt at wrapping the libusb library into a Go-like binding.

Supported platforms include:

- linux
- darwin
- windows

[ciimg]:  https://travis-ci.org/kylelemons/gousb.svg?branch=master
[ci]:     https://travis-ci.org/kylelemons/gousb
[docimg]: https://godoc.org/github.com/kylelemons/gousb?status.svg
[doc]:    https://godoc.org/github.com/kylelemons/gousb

Contributing
============
Because I am a Google employee, contributing to this project will require signing the [Google CLA][cla].
This is the same agreement that is required for contributing to Go itself, so if you have
already filled it out for that, you needn't fill it out again.
You will need to send me the email address that you used to sign the agreement
so that I can verify that it is on file before I can accept pull requests.

[cla]: https://cla.developers.google.com/

Installation
============

Dependencies
------------
You must first install [libusb-1.0](http://libusb.org/wiki/libusb-1.0).  This is pretty straightforward on linux and darwin.  The cgo package should be able to find it if you install it in the default manner or use your distribution's package manager.  How to tell cgo how to find one installed in a non-default place is beyond the scope of this README.

*Note*: If you are installing this on darwin, you will probably need to run `fixlibusb_darwin.sh /usr/local/lib/libusb-1.0/libusb.h` because of an LLVM incompatibility.  It shouldn't break C programs, though I haven't tried it in anger.

Example: lsusb
--------------
The gousb project provides a simple but useful example: lsusb.  This binary will list the USB devices connected to your system and various interesting tidbits about them, their configurations, endpoints, etc.  To install it, run the following command:

    go get -v github.com/kylelemons/gousb/lsusb

gousb
-----
If you installed the lsusb example, both libraries below are already installed.

Installing the primary gousb package is really easy:

    go get -v github.com/kylelemons/gousb/usb

There is also a `usbid` package that will not be installed by default by this command, but which provides useful information including the human-readable vendor and product codes for detected hardware.  It's not installed by default and not linked into the `usb` package by default because it adds ~400kb to the resulting binary.  If you want both, they can be installed thus:

    go get -v github.com/kylelemons/gousb/usb{,id}

Notes for installation on Windows
---------------------------------

You'll need:

- Gcc - tested on [Win-Builds](http://win-builds.org/) and MSYS/MINGW
- pkg-config - see http://www.mingw.org/wiki/FAQ, "How do I get pkg-config installed?"
- [libusb-1.0](http://sourceforge.net/projects/libusb/files/libusb-1.0/).

Make sure the `libusb-1.0.pc` pkg-config file from libusb was installed
and that the result of the `pkg-config --cflags libusb-1.0` command shows the
correct include path for installed libusb.

After that you can continue with instructions for lsusb/gousb above.

Documentation
=============
The documentation can be viewed via local godoc or via the excellent [godoc.org](http://godoc.org/):

- [usb](http://godoc.org/github.com/kylelemons/gousb/usb)
- [usbid](http://godoc.org/pkg/github.com/kylelemons/gousb/usbid)
