//go:build linux || darwin

package gpuagent

import (
	"errors"
	"os"
	"syscall"
)

func privateMode(st os.FileInfo) bool { return st.Mode().Perm()&0077 == 0 }
func replaceConfig(a, b string) error { return os.Rename(a, b) }
func lock(path string) (func(), error) {
	f, e := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	if e = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		f.Close()
		return nil, errors.New("GPU agent already running")
	}
	return func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN); _ = f.Close() }, nil
}
