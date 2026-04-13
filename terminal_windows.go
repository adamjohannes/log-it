//go:build windows

package logger

// isTerminal reports whether the given file descriptor is a terminal.
// On Windows, this always returns false to default to JSON output.
func isTerminal(fd uintptr) bool {
	return false
}
