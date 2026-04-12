package logger

import (
	"errors"
	"fmt"
)

// enrichErrors scans fields for values implementing the error interface
// and enriches them with structured error information:
//   - The original key is replaced with the error's message string
//   - A "<key>_type" field is added with the concrete type (e.g., "*os.PathError")
//   - A "<key>_chain" field is added with the unwrap chain (only if wrapped)
//
// Non-error fields are passed through unchanged. Returns the original map
// if no error values are found (zero allocation in the common case).
func enrichErrors(fields map[string]any) map[string]any {
	if len(fields) == 0 {
		return fields
	}
	var enriched map[string]any
	for k, v := range fields {
		err, ok := v.(error)
		if !ok {
			continue
		}
		if enriched == nil {
			enriched = make(map[string]any, len(fields)+3)
			for fk, fv := range fields {
				enriched[fk] = fv
			}
		}
		enriched[k] = err.Error()
		enriched[k+"_type"] = fmt.Sprintf("%T", err)

		chain := []string{err.Error()}
		for inner := errors.Unwrap(err); inner != nil; inner = errors.Unwrap(inner) {
			chain = append(chain, inner.Error())
		}
		if len(chain) > 1 {
			enriched[k+"_chain"] = chain
		}
	}
	if enriched != nil {
		return enriched
	}
	return fields
}
