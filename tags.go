package setay

import (
	"reflect"
	"strings"
)

// tagOptions holds the parsed struct tag information.
type tagOptions struct {
	name      string
	omitempty bool
	skip      bool // setay:"-"
}

// parseTag parses a "setay" struct tag.
// Examples: `setay:"name"`, `setay:"name,omitempty"`, `setay:"-"`
func parseTag(field reflect.StructField) tagOptions {
	tag := field.Tag.Get("setay")
	if tag == "" {
		return tagOptions{name: fieldNameToKey(field.Name)}
	}
	if tag == "-" {
		return tagOptions{skip: true}
	}

	parts := strings.Split(tag, ",")
	opts := tagOptions{}

	if parts[0] != "" {
		opts.name = parts[0]
	} else {
		opts.name = fieldNameToKey(field.Name)
	}

	for _, part := range parts[1:] {
		if part == "omitempty" {
			opts.omitempty = true
		}
	}

	return opts
}

// fieldNameToKey converts a Go struct field name to a setay key name.
// PascalCase → snake_case-ish: just lowercase the first letter.
// Users who want specific names should use struct tags.
func fieldNameToKey(name string) string {
	if name == "" {
		return ""
	}
	// Use the field name as-is (camelCase/PascalCase preserved).
	// This matches encoding/json behavior when no tag is specified.
	return name
}

// isZeroValue checks if a reflect.Value is the zero value for its type.
func isZeroValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	case reflect.Struct:
		// For time.Time, check IsZero()
		if v.Type().String() == "time.Time" {
			return v.MethodByName("IsZero").Call(nil)[0].Bool()
		}
		return false
	}
	return false
}
