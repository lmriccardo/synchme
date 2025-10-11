//go:build linux || darwin || freebsd

package utils

import (
	"os"
	"syscall"
)

// isFileOpenUnix checks if a file is locked on Unix-like systems.
func isFileOpen(path string) bool {
	f, err := os.OpenFile(path, os.O_RDWR, 0666)
	if err != nil {
		return true // cannot open, possibly locked or missing
	}

	defer func() {
		if err := f.Close(); err != nil {
			ERROR("Error when closing ", path, ": ", err)
		}
	}()

	// Try to acquire an exclusive non-blocking lock
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return true // locked by another process
	}

	defer func() {
		if err := syscall.Flock(int(f.Fd()), syscall.LOCK_UN); err != nil {
			ERROR("Error when calling Flock: ", err)
		}
	}()

	return false
}
