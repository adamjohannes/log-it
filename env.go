package logger

import (
	"os"
	"strings"
)

// WithEnvConfig reads LOG_LEVEL and LOG_FORMAT environment variables
// and applies them to the logger configuration. This is an explicit
// opt-in option — it does nothing magical at init time.
//
// Supported values:
//   - LOG_LEVEL: trace, debug, info, warning, error, fatal (case-insensitive)
//   - LOG_FORMAT: json, text, logfmt (case-insensitive)
func WithEnvConfig() Option {
	return func(l *Logger) {
		if lvl := os.Getenv("LOG_LEVEL"); lvl != "" {
			if parsed, ok := parseLevel(lvl); ok {
				l.minLevel.Store(int32(parsed))
			}
		}
		if fmt := os.Getenv("LOG_FORMAT"); fmt != "" {
			switch strings.ToLower(fmt) {
			case "json":
				l.formatter = JSONFormatter{}
			case "text":
				l.formatter = TextFormatter{}
			case "logfmt":
				l.formatter = LogfmtFormatter{}
			}
		}
	}
}
