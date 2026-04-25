package logger

import "strings"

// Level represents a log severity level.
type Level int

const (
	TRACE Level = iota
	DEBUG
	INFO
	WARNING
	ERROR
	FATAL
)

func (l Level) String() string {
	switch l {
	case TRACE:
		return "TRACE"
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARNING:
		return "WARNING"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// parseLevel converts a string to a Level. Case-insensitive.
func parseLevel(s string) (Level, bool) {
	switch strings.ToLower(s) {
	case "trace":
		return TRACE, true
	case "debug":
		return DEBUG, true
	case "info":
		return INFO, true
	case "warning", "warn":
		return WARNING, true
	case "error":
		return ERROR, true
	case "fatal":
		return FATAL, true
	default:
		return INFO, false
	}
}
