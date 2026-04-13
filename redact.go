package logger

// WithRedactFields returns an Option that installs a middleware to
// replace the values of the named fields with "[REDACTED]". Field
// matching is exact and applies recursively to nested maps (Group
// fields). ERROR and FATAL entries are still redacted — sensitive
// data must never leak regardless of severity.
func WithRedactFields(fields ...string) Option {
	return WithRedactFieldsFunc("[REDACTED]", fields...)
}

// WithRedactFieldsFunc is like WithRedactFields but allows a custom
// replacement string (e.g., "***", "<removed>").
func WithRedactFieldsFunc(replacement string, fields ...string) Option {
	set := make(map[string]struct{}, len(fields))
	for _, f := range fields {
		set[f] = struct{}{}
	}
	return WithMiddleware(func(entry map[string]any) map[string]any {
		redactMap(entry, set, replacement)
		return entry
	})
}

func redactMap(m map[string]any, fields map[string]struct{}, replacement string) {
	for k, v := range m {
		if _, redact := fields[k]; redact {
			m[k] = replacement
			continue
		}
		// Recurse into nested maps (Group fields)
		if nested, ok := v.(map[string]any); ok {
			redactMap(nested, fields, replacement)
		}
	}
}
