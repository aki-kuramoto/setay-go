package setay

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"time"
)

// toTimer is the interface implemented by types that can convert themselves
// to a time.Time. All wantai timestamp types (UtcNanoTs, UtcMilliTs, etc.)
// implement this interface.
type toTimer interface {
	ToTime() time.Time
}

// Marshal returns the setay encoding of v.
// v must be a struct, map[string]T, or a pointer to one of those.
func Marshal(v interface{}) ([]byte, error) {
	return MarshalIndent(v, "\t")
}

// MarshalIndent returns the setay encoding of v with the specified indent string.
func MarshalIndent(v interface{}, indent string) ([]byte, error) {
	m := &marshaler{indent: indent}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return []byte("null\n"), nil
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct && rv.Kind() != reflect.Map {
		return nil, fmt.Errorf("setay: cannot marshal %s (must be struct or map)", rv.Kind())
	}
	m.marshalTopLevel(rv)
	return []byte(m.out.String()), nil
}

// MarshalFile marshals v and writes it to a file.
func MarshalFile(filename string, v interface{}) error {
	data, err := Marshal(v)
	if err != nil {
		return err
	}
	return writeFileAtomic(filename, data)
}

type marshaler struct {
	out    strings.Builder
	indent string
}

func (m *marshaler) marshalTopLevel(v reflect.Value) {
	m.out.WriteString("{\n")
	m.marshalDictBody(v, 1)
	m.out.WriteString("}\n")
}

func (m *marshaler) marshalDictBody(v reflect.Value, depth int) {
	switch v.Kind() {
	case reflect.Struct:
		m.marshalStructFields(v, depth)
	case reflect.Map:
		m.marshalMapFields(v, depth)
	}
}

func (m *marshaler) marshalStructFields(v reflect.Value, depth int) {
	t := v.Type()
	first := true
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		opts := parseTag(field)
		if opts.skip {
			continue
		}
		fv := v.Field(i)
		if opts.omitempty && isZeroValue(fv) {
			continue
		}

		if !first {
			// Previous entry already ended with ";\n"
		}
		first = false

		m.writeIndent(depth)
		m.writeKey(opts.name)
		m.marshalFieldValue(fv, depth)
		m.out.WriteString(";\n")
	}
}

func (m *marshaler) marshalMapFields(v reflect.Value, depth int) {
	keys := v.MapKeys()
	for i, key := range keys {
		_ = i
		m.writeIndent(depth)
		m.writeKey(fmt.Sprintf("%v", key.Interface()))
		m.marshalFieldValue(v.MapIndex(key), depth)
		m.out.WriteString(";\n")
	}
}

func (m *marshaler) marshalFieldValue(v reflect.Value, depth int) {
	// Unwrap interface
	if v.Kind() == reflect.Interface {
		if v.IsNil() {
			m.out.WriteString(" = null")
			return
		}
		v = v.Elem()
	}

	// Unwrap pointer
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			m.out.WriteString(" = null")
			return
		}
		v = v.Elem()
	}

	// Check for time.Time
	if v.Type().String() == "time.Time" {
		t := v.Interface().(time.Time)
		m.out.WriteString(" = UtcTs(\"")
		m.out.WriteString(t.UTC().Format("2006-01-02 15:04:05"))
		m.out.WriteString("\")")
		return
	}

	// Check for wantai timestamp types (types implementing ToTime() time.Time)
	if tt, ok := v.Interface().(toTimer); ok {
		t := tt.ToTime()
		m.out.WriteString(" = UtcTs(\"")
		m.out.WriteString(t.UTC().Format("2006-01-02 15:04:05"))
		m.out.WriteString("\")")
		return
	}

	switch v.Kind() {
	case reflect.String:
		m.out.WriteString(" = ")
		m.writeString(v.String())
	case reflect.Bool:
		if v.Bool() {
			m.out.WriteString(" = true")
		} else {
			m.out.WriteString(" = false")
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		m.out.WriteString(fmt.Sprintf(" = %d", v.Int()))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		m.out.WriteString(fmt.Sprintf(" = %d", v.Uint()))
	case reflect.Float32, reflect.Float64:
		f := v.Float()
		if f == math.Trunc(f) && !math.IsInf(f, 0) && !math.IsNaN(f) {
			// Integer-valued float: emit as "N.0"
			m.out.WriteString(fmt.Sprintf(" = %.1f", f))
		} else {
			m.out.WriteString(fmt.Sprintf(" = %g", f))
		}
	case reflect.Slice, reflect.Array:
		m.marshalSlice(v, depth)
	case reflect.Struct:
		m.marshalNestedDict(v, depth)
	case reflect.Map:
		m.marshalNestedDict(v, depth)
	default:
		m.out.WriteString(fmt.Sprintf(" = \"%v\"", v.Interface()))
	}
}

func (m *marshaler) marshalSlice(v reflect.Value, depth int) {
	if v.IsNil() || v.Len() == 0 {
		m.out.WriteString(" = []")
		return
	}

	// Decide single-line vs multi-line
	allSimple := true
	for i := 0; i < v.Len(); i++ {
		elem := v.Index(i)
		if elem.Kind() == reflect.Interface || elem.Kind() == reflect.Ptr {
			if !elem.IsNil() {
				elem = elem.Elem()
			}
		}
		k := elem.Kind()
		if k == reflect.Struct || k == reflect.Map || k == reflect.Slice || k == reflect.Array {
			if elem.Type().String() != "time.Time" {
				allSimple = false
				break
			}
		}
	}

	if allSimple && v.Len() <= 10 {
		// Try single-line
		m.out.WriteString(" = ")
		m.marshalSliceWithoutEquals(v, depth, true)
	} else {
		m.out.WriteString(" =\n")
		m.writeIndent(depth)
		m.marshalSliceWithoutEquals(v, depth, false)
	}
}

func (m *marshaler) marshalSliceWithoutEquals(v reflect.Value, depth int, allSimple bool) {
	if v.IsNil() || v.Len() == 0 {
		m.out.WriteString("[]")
		return
	}
	if allSimple && v.Len() <= 10 {
		// Try single-line
		m.out.WriteString("[ ")
		for i := 0; i < v.Len(); i++ {
			if i > 0 {
				m.out.WriteString(", ")
			}
			m.marshalInlineValue(v.Index(i), depth)
		}
		m.out.WriteString(" ]")
	} else {
		m.out.WriteString("[\n")
		for i := 0; i < v.Len(); i++ {
			m.writeIndent(depth + 1)
			m.marshalInlineValue(v.Index(i), depth+1)
			m.out.WriteString(",\n")
		}
		m.writeIndent(depth)
		m.out.WriteByte(']')
	}
}

func (m *marshaler) marshalNestedDict(v reflect.Value, depth int) {
	m.out.WriteString(" =\n")
	m.writeIndent(depth)
	m.marshalBlockWithoutEquals(v, depth)
}

func (m *marshaler) marshalBlockWithoutEquals(v reflect.Value, depth int) {
	m.out.WriteString("{\n")
	m.marshalDictBody(v, depth+1)
	m.writeIndent(depth)
	m.out.WriteByte('}')
}

func (m *marshaler) marshalInlineValue(v reflect.Value, depth int) {
	if v.Kind() == reflect.Interface {
		if v.IsNil() {
			m.out.WriteString("null")
			return
		}
		v = v.Elem()
	}
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			m.out.WriteString("null")
			return
		}
		v = v.Elem()
	}

	if v.Type().String() == "time.Time" {
		t := v.Interface().(time.Time)
		m.out.WriteString("UtcTs(\"")
		m.out.WriteString(t.UTC().Format("2006-01-02 15:04:05"))
		m.out.WriteString("\")")
		return
	}

	// Check for wantai timestamp types (types implementing ToTime() time.Time)
	if tt, ok := v.Interface().(toTimer); ok {
		t := tt.ToTime()
		m.out.WriteString("UtcTs(\"")
		m.out.WriteString(t.UTC().Format("2006-01-02 15:04:05"))
		m.out.WriteString("\")")
		return
	}

	switch v.Kind() {
	case reflect.String:
		m.writeString(v.String())
	case reflect.Bool:
		if v.Bool() {
			m.out.WriteString("true")
		} else {
			m.out.WriteString("false")
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		m.out.WriteString(fmt.Sprintf("%d", v.Int()))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		m.out.WriteString(fmt.Sprintf("%d", v.Uint()))
	case reflect.Float32, reflect.Float64:
		f := v.Float()
		if f == math.Trunc(f) && !math.IsInf(f, 0) && !math.IsNaN(f) {
			m.out.WriteString(fmt.Sprintf("%.1f", f))
		} else {
			m.out.WriteString(fmt.Sprintf("%g", f))
		}
	case reflect.Struct, reflect.Map:
		m.marshalBlockWithoutEquals(v, depth)
	case reflect.Slice, reflect.Array:
		// Re-evaluate allSimple for inner slice
		allSimple := true
		for i := 0; i < v.Len(); i++ {
			elem := v.Index(i)
			if elem.Kind() == reflect.Interface || elem.Kind() == reflect.Ptr {
				if !elem.IsNil() {
					elem = elem.Elem()
				}
			}
			k := elem.Kind()
			if k == reflect.Struct || k == reflect.Map || k == reflect.Slice || k == reflect.Array {
				if elem.Type().String() != "time.Time" {
					allSimple = false
					break
				}
			}
		}
		m.marshalSliceWithoutEquals(v, depth, allSimple)
	default:
		m.out.WriteString(fmt.Sprintf("\"%v\"", v.Interface()))
	}
}

// writeKey writes a dict key. Uses bare key if valid, otherwise quoted.
func (m *marshaler) writeKey(key string) {
	if isValidBareKey(key) {
		m.out.WriteString(key)
	} else {
		m.writeString(key)
	}
}

// isValidBareKey checks if a string can be used as an unquoted key.
func isValidBareKey(s string) bool {
	if len(s) == 0 {
		return false
	}
	// First char: [a-zA-Z_]
	if !isAlpha(s[0]) && s[0] != '_' {
		return false
	}
	// Last char: [a-zA-Z0-9_] (no hyphen)
	if len(s) > 1 {
		last := s[len(s)-1]
		if !isAlpha(last) && !isDigit(last) && last != '_' {
			return false
		}
	}
	// Middle chars: [a-zA-Z0-9_-]
	for i := 1; i < len(s)-1; i++ {
		c := s[i]
		if !isAlpha(c) && !isDigit(c) && c != '_' && c != '-' {
			return false
		}
	}
	return true
}

func isAlpha(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }
func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// writeString writes a double-quoted string with proper escaping.
func (m *marshaler) writeString(s string) {
	m.out.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			m.out.WriteString(`\"`)
		case '\\':
			m.out.WriteString(`\\`)
		case '\n':
			m.out.WriteString(`\n`)
		case '\r':
			m.out.WriteString(`\r`)
		case '\t':
			m.out.WriteString(`\t`)
		case '\x00':
			m.out.WriteString(`\0`)
		default:
			if r < 0x20 {
				m.out.WriteString(fmt.Sprintf(`\x%02X`, r))
			} else {
				m.out.WriteRune(r)
			}
		}
	}
	m.out.WriteByte('"')
}

func (m *marshaler) writeIndent(depth int) {
	for i := 0; i < depth; i++ {
		m.out.WriteString(m.indent)
	}
}
