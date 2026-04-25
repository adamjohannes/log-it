package logger

import "time"

// FieldType represents the type of a Field value.
type FieldType uint8

// FieldType values identify the stored type inside a Field, allowing
// Value() to return the concrete value without a type switch on interface{}.
const (
	FieldTypeAny FieldType = iota
	FieldTypeString
	FieldTypeInt
	FieldTypeInt64
	FieldTypeFloat64
	FieldTypeBool
	FieldTypeError
	FieldTypeDuration
)

// Field is a pre-typed key-value pair that avoids boxing primitive
// values into interface{}, reducing heap allocations at the call site.
type Field struct {
	Key       string
	Type      FieldType
	Integer   int64
	Float     float64
	Str       string
	Interface any
}

// Value returns the field's value as an any for map insertion.
func (f Field) Value() any {
	switch f.Type {
	case FieldTypeString:
		return f.Str
	case FieldTypeInt, FieldTypeInt64:
		return f.Integer
	case FieldTypeFloat64:
		return f.Float
	case FieldTypeBool:
		return f.Integer != 0
	case FieldTypeError:
		if f.Interface == nil {
			return nil
		}
		return f.Interface
	case FieldTypeDuration:
		return time.Duration(f.Integer).String()
	default:
		return f.Interface
	}
}

// --- Typed field constructors ---

// String creates a string Field.
func String(key, val string) Field {
	return Field{Key: key, Type: FieldTypeString, Str: val}
}

// Int creates an int Field.
func Int(key string, val int) Field {
	return Field{Key: key, Type: FieldTypeInt, Integer: int64(val)}
}

// Int64 creates an int64 Field.
func Int64(key string, val int64) Field {
	return Field{Key: key, Type: FieldTypeInt64, Integer: val}
}

// Float64 creates a float64 Field.
func Float64(key string, val float64) Field {
	return Field{Key: key, Type: FieldTypeFloat64, Float: val}
}

// Bool creates a bool Field.
func Bool(key string, val bool) Field {
	f := Field{Key: key, Type: FieldTypeBool}
	if val {
		f.Integer = 1
	}
	return f
}

// Err creates an error Field with key "error".
func Err(err error) Field {
	return Field{Key: "error", Type: FieldTypeError, Interface: err}
}

// Duration creates a duration Field stored as a human-readable string.
func Duration(key string, val time.Duration) Field {
	return Field{Key: key, Type: FieldTypeDuration, Integer: int64(val)}
}

// Any creates a Field with an arbitrary value (fallback).
func Any(key string, val any) Field {
	return Field{Key: key, Type: FieldTypeAny, Interface: val}
}

// Group creates a Field containing nested key-value pairs.
// The value is serialized as a nested JSON object.
func Group(key string, fields ...Field) Field {
	m := make(map[string]any, len(fields))
	for _, f := range fields {
		m[f.Key] = f.Value()
	}
	return Field{Key: key, Type: FieldTypeAny, Interface: m}
}

// fieldsToMap converts a slice of typed Fields to a map for internal use.
func fieldsToMap(fields []Field) map[string]any {
	if len(fields) == 0 {
		return nil
	}
	m := make(map[string]any, len(fields))
	for _, f := range fields {
		m[f.Key] = f.Value()
	}
	return m
}
