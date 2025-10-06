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
	defer f.Close()

	// Try to acquire an exclusive non-blocking lock
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return true // locked by another process
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	return false
}
