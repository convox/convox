//go:build !hidraw && !android && cgo

package hid

/*
#include "hidapi/libusb/hidapi_libusb.h"
*/
import "C"

import (
	_ "github.com/bearsh/hid/libusb/libusb/os/udev"
)
