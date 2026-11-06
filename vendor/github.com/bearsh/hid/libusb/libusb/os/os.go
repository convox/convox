//go:build !hidraw && linux && cgo

package os

/*
#cgo CFLAGS: -I. -I../. -I../../../. -DDEFAULT_VISIBILITY="" -DOS_LINUX -D_GNU_SOURCE -DPLATFORM_POSIX
#cgo logging CFLAGS: -DENABLE_LOGGING -DENABLE_DEBUG_LOGGING -DUSE_SYSTEM_LOGGING_FACILITY
*/
import "C"
