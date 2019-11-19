package azfile

import (
	"os"
	"reflect"
	"unsafe"

	"golang.org/x/sys/windows"
)

type mmf []byte

func newMMF(file *os.File, writable bool, offset int64, length int) (mmf, error) {
	prot, access := uint32(windows.PAGE_READONLY), uint32(windows.FILE_MAP_READ) // Assume read-only
	if writable {
		prot, access = uint32(windows.PAGE_READWRITE), uint32(windows.FILE_MAP_WRITE)
	}
	maxSize := int64(offset + int64(length))
	hMMF, errno := windows.CreateFileMapping(windows.Handle(file.Fd()), nil, prot, uint32(maxSize>>32), uint32(maxSize&0xffffffff), nil)
	if hMMF == 0 {
		return nil, os.NewSyscallError("CreateFileMapping", errno)
	}
	defer windows.CloseHandle(hMMF)
	addr, errno := windows.MapViewOfFile(hMMF, access, uint32(offset>>32), uint32(offset&0xffffffff), uintptr(length))
	m := mmf{}
	h := (*reflect.SliceHeader)(unsafe.Pointer(&m))
	h.Data = addr
	h.Len = length
	h.Cap = h.Len
	return m, nil
}

func (m *mmf) unmap() {
	addr := uintptr(unsafe.Pointer(&(([]byte)(*m)[0])))
	*m = mmf{}
	err := windows.UnmapViewOfFile(addr)
	if err != nil {
		sanityCheckFailed(err.Error())
	}
}
