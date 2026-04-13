package logger

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"time"
	"unicode/utf8"
)

// appendJSONEntry encodes a map[string]any as a JSON object into dst.
// Keys are sorted for deterministic output. Falls back to encoding/json
// for types that cannot be encoded directly.
func appendJSONEntry(dst []byte, entry map[string]any) []byte {
	keys := make([]string, 0, len(entry))
	for k := range entry {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	dst = append(dst, '{')
	for i, k := range keys {
		if i > 0 {
			dst = append(dst, ',')
		}
		dst = appendJSONString(dst, k)
		dst = append(dst, ':')
		dst = appendJSONValue(dst, entry[k])
	}
	dst = append(dst, '}')
	return dst
}

// appendJSONValue encodes a single value as JSON.
func appendJSONValue(dst []byte, v any) []byte {
	switch val := v.(type) {
	case nil:
		return append(dst, "null"...)
	case string:
		return appendJSONString(dst, val)
	case bool:
		return strconv.AppendBool(dst, val)
	case int:
		return strconv.AppendInt(dst, int64(val), 10)
	case int8:
		return strconv.AppendInt(dst, int64(val), 10)
	case int16:
		return strconv.AppendInt(dst, int64(val), 10)
	case int32:
		return strconv.AppendInt(dst, int64(val), 10)
	case int64:
		return strconv.AppendInt(dst, val, 10)
	case uint:
		return strconv.AppendUint(dst, uint64(val), 10)
	case uint8:
		return strconv.AppendUint(dst, uint64(val), 10)
	case uint16:
		return strconv.AppendUint(dst, uint64(val), 10)
	case uint32:
		return strconv.AppendUint(dst, uint64(val), 10)
	case uint64:
		return strconv.AppendUint(dst, val, 10)
	case float32:
		return appendJSONFloat(dst, float64(val))
	case float64:
		return appendJSONFloat(dst, val)
	case error:
		return appendJSONString(dst, val.Error())
	case time.Duration:
		return appendJSONString(dst, val.String())
	case json.Number:
		return append(dst, val.String()...)
	case map[string]any:
		return appendJSONEntry(dst, val)
	case []string:
		dst = append(dst, '[')
		for i, s := range val {
			if i > 0 {
				dst = append(dst, ',')
			}
			dst = appendJSONString(dst, s)
		}
		return append(dst, ']')
	case []any:
		dst = append(dst, '[')
		for i, elem := range val {
			if i > 0 {
				dst = append(dst, ',')
			}
			dst = appendJSONValue(dst, elem)
		}
		return append(dst, ']')
	default:
		// Fallback to encoding/json for unknown types
		b, err := json.Marshal(val)
		if err != nil {
			return appendJSONString(dst, fmt.Sprintf("%v", val))
		}
		return append(dst, b...)
	}
}

// appendJSONFloat encodes a float64 as JSON, handling special values.
func appendJSONFloat(dst []byte, f float64) []byte {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		// JSON does not support NaN/Inf; encode as string
		return appendJSONString(dst, strconv.FormatFloat(f, 'f', -1, 64))
	}
	return strconv.AppendFloat(dst, f, 'f', -1, 64)
}

// appendJSONString encodes a string as a quoted JSON string, escaping
// control characters and invalid UTF-8.
func appendJSONString(dst []byte, s string) []byte {
	dst = append(dst, '"')
	for i := 0; i < len(s); {
		b := s[i]
		if b < utf8.RuneSelf {
			switch {
			case b == '"':
				dst = append(dst, '\\', '"')
			case b == '\\':
				dst = append(dst, '\\', '\\')
			case b == '\n':
				dst = append(dst, '\\', 'n')
			case b == '\r':
				dst = append(dst, '\\', 'r')
			case b == '\t':
				dst = append(dst, '\\', 't')
			case b < 0x20:
				// Other control characters
				dst = append(dst, '\\', 'u', '0', '0')
				dst = append(dst, hexDigit(b>>4), hexDigit(b&0x0f))
			default:
				dst = append(dst, b)
			}
			i++
		} else {
			r, size := utf8.DecodeRuneInString(s[i:])
			if r == utf8.RuneError && size == 1 {
				// Invalid UTF-8 byte
				dst = append(dst, '\\', 'u', 'f', 'f', 'f', 'd')
			} else {
				dst = append(dst, s[i:i+size]...)
			}
			i += size
		}
	}
	dst = append(dst, '"')
	return dst
}

func hexDigit(b byte) byte {
	const hex = "0123456789abcdef"
	return hex[b&0x0f]
}
