//go:build linux

package logger

import "syscall"

// isTerminal reports whether the given file descriptor is a terminal.
func isTerminal(fd uintptr) bool {
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, syscall.TCGETS, 0)
	return err == 0
}
