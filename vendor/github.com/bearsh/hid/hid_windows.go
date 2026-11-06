//go:build cgo

package hid

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"

	_ "github.com/bearsh/hid/hidapi"
	_ "github.com/bearsh/hid/hidapi/windows"
)

/*
#include "hidapi/windows/hidapi_winapi.h"
*/
import "C"

// GetContainerId gets the container ID for a HID device.
// This function is windows specific.
func (dev *Device) GetContainerId() (*windows.GUID, error) {
	// Abort if device closed in between
	dev.lock.Lock()
	device := dev.device
	dev.lock.Unlock()

	if device == nil {
		return nil, ErrDeviceClosed
	}

	var c_guid C.GUID

	res := int(C.hid_winapi_get_container_id(device, &c_guid))

	if res < 0 {
		return nil, errors.New("hidapi: get container id")
	}

	guid := &windows.GUID{
		Data1: uint32(c_guid.Data1),
		Data2: uint16(c_guid.Data2),
		Data3: uint16(c_guid.Data3),
	}

	C.memcpy(unsafe.Pointer(&guid.Data4[0]), unsafe.Pointer(&c_guid.Data4[0]), C.size_t(len(guid.Data4)))

	return guid, nil
}
