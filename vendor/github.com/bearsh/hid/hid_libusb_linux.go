//go:build !hidraw && cgo

package hid

/*
#include "hidapi/libusb/hidapi_libusb.h"
*/
import "C"

import (
	"errors"
	"unsafe"

	_ "github.com/bearsh/hid/hidapi"
	_ "github.com/bearsh/hid/hidapi/libusb"
	_ "github.com/bearsh/hid/libusb/libusb"
	_ "github.com/bearsh/hid/libusb/libusb/os"
)

// WrapSysDevice opens a HID device using a platform-specific file descriptor that can be recognized by libusb
func WrapSysDevice(sysDev int64, ifNumber int) (*Device, error) {
	device := C.hid_libusb_wrap_sys_device(C.intptr_t(sysDev), C.int(ifNumber))
	if device == nil {
		return nil, errors.New("hidapi: failed to open device")
	}

	return &Device{
		device: device,
	}, nil
}

// in above function, the correct argument type for sysDev would be a uintptr but generating
// java binding does not support uintptr. so we add a 'static' check to make sure the size
// of an int is not smaller than the size of an uintptr.
// see also https://commaok.xyz/post/compile-time-assertions/
func uintToSmall()

func init() {
	if unsafe.Sizeof(int64(0)) < unsafe.Sizeof(C.intptr_t(0)) {
		uintToSmall()
	}
}
