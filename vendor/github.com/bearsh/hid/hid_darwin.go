//go:build !ios && cgo

package hid

import (
	_ "github.com/bearsh/hid/hidapi"
	_ "github.com/bearsh/hid/hidapi/mac"
)
