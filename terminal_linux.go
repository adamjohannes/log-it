//go:build linux

package logger

import (
	"syscall"
	"unsafe"
)

// isTerminal reports whether the given file descriptor is a terminal.
func isTerminal(fd uintptr) bool {
	var termios syscall.Termios
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, syscall.TCGETS, uintptr(unsafe.Pointer(&termios)))
	return err == 0
}
