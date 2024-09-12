//go:build !hidraw && linux && cgo

package libusb

/*
#cgo CFLAGS: -I../. -I../../libusb/libusb
#cgo logging CFLAGS: -DDEBUG_PRINTF
#cgo android,logging LDFLAGS: -llog
*/
import "C"
