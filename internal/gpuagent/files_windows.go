package gpuagent

import (
	"errors"
	"os"
	"syscall"
	"unsafe"
)

func privateMode(st os.FileInfo) bool { return true } // Windows installer applies a private NTFS ACL.
func replaceConfig(a, b string) error {
	from, e := syscall.UTF16PtrFromString(a)
	if e != nil {
		return e
	}
	to, e := syscall.UTF16PtrFromString(b)
	if e != nil {
		return e
	}
	p := syscall.NewLazyDLL("kernel32.dll").NewProc("MoveFileExW")
	r, _, e := p.Call(uintptr(unsafe.Pointer(from)), uintptr(unsafe.Pointer(to)), 0x1|0x8)
	if r == 0 {
		return e
	}
	return nil
}
func lock(path string) (func(), error) {
	f, e := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	p := syscall.NewLazyDLL("kernel32.dll").NewProc("LockFileEx")
	var overlap syscall.Overlapped
	r, _, _ := p.Call(f.Fd(), 0x1|0x2, 0, 1, 0, uintptr(unsafe.Pointer(&overlap)))
	if r == 0 {
		f.Close()
		return nil, errors.New("GPU agent already running")
	}
	return func() { f.Close() }, nil
}
