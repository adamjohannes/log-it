//go:build !windows

package logger

import "syscall"

// isTerminal reports whether the given file descriptor is a terminal.
func isTerminal(fd uintptr) bool {
	// TIOCGETA is the ioctl request code for getting terminal attributes
	// on Darwin and most BSDs. On Linux it maps to TCGETS.
	const ioctlReadTermios = syscall.TIOCGETA
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, ioctlReadTermios, 0)
	return err == 0
}
