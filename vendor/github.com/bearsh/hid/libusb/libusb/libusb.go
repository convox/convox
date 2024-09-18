//go:build !hidraw && linux && cgo

package libusb

/*
#cgo CFLAGS: -I../.. -DDEFAULT_VISIBILITY="" -DOS_LINUX -D_GNU_SOURCE -DPLATFORM_POSIX
#cgo !hidraw,linux,noiconv CFLAGS: -DNO_ICONV
#cgo logging CFLAGS: -DENABLE_LOGGING -DENABLE_DEBUG_LOGGING -DUSE_SYSTEM_LOGGING_FACILITY
#cgo android,logging LDFLAGS: -llog

//#include "os/events_posix.c"
//#include "os/threads_posix.c"

//#include "os/linux_usbfs.c"
//#include "os/linux_netlink.c"
*/
import "C"
