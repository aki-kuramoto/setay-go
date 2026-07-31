package setay_test

import (
	"reflect"
	"testing"

	setay "github.com/aki-kuramoto/setay-go"
)

// Hexadecimal literals whose digits include 'e'/'E' (e.g. 0xE, 0xBEEF) used to
// be misclassified as floats and fail with a strconv.ParseFloat error when the
// target was an interface{}. They must now decode to int64 like any other hex.
func TestHexIntoAny(t *testing.T) {
	cases := map[string]int64{
		"0x10":   0x10,
		"0xFF":   0xFF,
		"0xE":    0xE,
		"0xe":    0xE,
		"0xFE":   0xFE,
		"0x1E":   0x1E,
		"0xBEEF": 0xBEEF,
		"0xdead": 0xdead,
		"-0xE":   -0xE,
		"0b1010": 0b1010,
		"0o17":   0o17,
		"0d42":   42,
		"42":     42,
	}
	for lit, want := range cases {
		t.Run(lit, func(t *testing.T) {
			var m map[string]any
			if err := setay.Unmarshal([]byte("{ v = "+lit+"; }"), &m); err != nil {
				t.Fatalf("Unmarshal(%s) error: %v", lit, err)
			}
			got, ok := m["v"].(int64)
			if !ok {
				t.Fatalf("%s: expected int64, got %T (%#v)", lit, m["v"], m["v"])
			}
			if got != want {
				t.Fatalf("%s: got %d, want %d", lit, got, want)
			}
		})
	}
}

// Floats must still decode to float64 into an interface{} target.
func TestFloatIntoAny(t *testing.T) {
	cases := map[string]float64{
		"3.14":   3.14,
		"1e5":    1e5,
		"1E5":    1e5,
		"2.5e-3": 2.5e-3,
	}
	for lit, want := range cases {
		t.Run(lit, func(t *testing.T) {
			var m map[string]any
			if err := setay.Unmarshal([]byte("{ v = "+lit+"; }"), &m); err != nil {
				t.Fatalf("Unmarshal(%s) error: %v", lit, err)
			}
			got, ok := m["v"].(float64)
			if !ok {
				t.Fatalf("%s: expected float64, got %T (%#v)", lit, m["v"], m["v"])
			}
			if got != want {
				t.Fatalf("%s: got %v, want %v", lit, got, want)
			}
		})
	}
}

// A mixed-type list (including hex with 'e' digits) decodes into []any without
// any dedicated union type — the original motivating use case.
func TestMixedHexListIntoAny(t *testing.T) {
	var m map[string]any
	if err := setay.Unmarshal([]byte(`{ data = [0x10, 0xE, 0xFF, 0xBEEF]; }`), &m); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	got, ok := m["data"].([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", m["data"])
	}
	want := []any{int64(0x10), int64(0xE), int64(0xFF), int64(0xBEEF)}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

// The scalar-or-list shape the user wanted: a key is sometimes a single hex
// value and sometimes a list of hex values, all read through interface{}.
func TestScalarOrHexListIntoAny(t *testing.T) {
	var single map[string]any
	if err := setay.Unmarshal([]byte(`{ key = 0xE; }`), &single); err != nil {
		t.Fatalf("scalar Unmarshal error: %v", err)
	}
	if single["key"] != int64(0xE) {
		t.Fatalf("scalar: got %#v, want int64(14)", single["key"])
	}

	var list map[string]any
	if err := setay.Unmarshal([]byte(`{ key = [0x10, 0x00, 0xEF]; }`), &list); err != nil {
		t.Fatalf("list Unmarshal error: %v", err)
	}
	if !reflect.DeepEqual(list["key"], []any{int64(0x10), int64(0x00), int64(0xEF)}) {
		t.Fatalf("list: got %#v", list["key"])
	}
}
