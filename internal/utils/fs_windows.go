//go:build windows

package utils

import "syscall"

// isFileOpenWindows checks if a file is locked on Windows.
func isFileOpen(path string) bool {
	ptr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return false
	}

	handle, err := syscall.CreateFile(
		ptr,
		syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		0,   // deny sharing
		nil, // no security attributes
		syscall.OPEN_EXISTING,
		0,
		0,
	)
	if err != nil {
		// File is likely locked or inaccessible
		return true
	}
	syscall.CloseHandle(handle)
	return false
}
