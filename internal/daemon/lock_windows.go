//go:build windows

package daemon

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

type FileLock struct {
	handle     windows.Handle
	overlapped windows.Overlapped
}

func AcquireLock(path string) (*FileLock, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(
		p,
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		0,
		nil,
		windows.OPEN_ALWAYS,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		if errors.Is(err, windows.ERROR_SHARING_VIOLATION) || errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			return nil, fmt.Errorf("daemon already running")
		}
		return nil, err
	}
	lock := &FileLock{handle: handle}
	err = windows.LockFileEx(
		handle,
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0,
		1,
		0,
		&lock.overlapped,
	)
	if err != nil {
		_ = windows.CloseHandle(handle)
		if errors.Is(err, windows.ERROR_SHARING_VIOLATION) || errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			return nil, fmt.Errorf("daemon already running")
		}
		return nil, err
	}
	return lock, nil
}

func (l *FileLock) Close() error {
	if l == nil || l.handle == 0 {
		return nil
	}
	_ = windows.UnlockFileEx(l.handle, 0, 1, 0, &l.overlapped)
	err := windows.CloseHandle(l.handle)
	l.handle = 0
	return err
}
