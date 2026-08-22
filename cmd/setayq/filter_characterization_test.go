package main

import (
	"reflect"
	"strings"
	"testing"
)

// These tests pin setayq's CURRENT filter behavior -- especially path handling
// (.field, [n], [], .["key"], ?, chaining) -- so that a later refactor that
// routes path parsing through the shared internal/setaypath grammar can be
// verified to preserve it exactly. They record what setayq does today; they are
// a regression net, not a fresh specification, and setayq had no tests before.

const charInput = "{\n" +
	"\tname = \"app\";\n" +
	"\tport = 8080;\n" +
	"\ttags = [ \"a\", \"b\", \"c\" ];\n" +
	"\tserver = { host = \"localhost\"; enabled = true };\n" +
	"\t\"a.b\" = 42;\n" +
	"}\n"

func evalToStrings(t *testing.T, filterExpr, input string) []string {
	t.Helper()
	fil, err := parseFilter(filterExpr)
	if err != nil {
		t.Fatalf("parseFilter(%q): %v", filterExpr, err)
	}
	val, err := parseSetayToValue(input, false)
	if err != nil {
		t.Fatalf("parseSetayToValue: %v", err)
	}
	results, err := evalFilter(fil, val, evalContext{})
	if err != nil {
		t.Fatalf("evalFilter(%q): %v", filterExpr, err)
	}
	out := make([]string, len(results))
	for i, r := range results {
		out[i] = valueToSetayString(r)
	}
	return out
}

func TestFilterCharacterizationValues(t *testing.T) {
	cases := []struct {
		filter string
		want   []string
	}{
		// Field access.
		{".name", []string{`"app"`}},
		{".port", []string{"8080"}},
		{".server.host", []string{`"localhost"`}},
		{".server.enabled", []string{"true"}},
		{".missing", []string{"null"}},
		{".a.b.c", []string{"null"}},
		// Index (including negative and out-of-range).
		{".tags[0]", []string{`"a"`}},
		{".tags[-1]", []string{`"c"`}},
		{".tags[9]", []string{"null"}},
		// Iterate.
		{".tags[]", []string{`"a"`, `"b"`, `"c"`}},
		{".tags[] | length", []string{"1", "1", "1"}},
		{".tags[] | .", []string{`"a"`, `"b"`, `"c"`}},
		// Quoted-key field access, both spellings.
		{`.["a.b"]`, []string{"42"}},
		{`."a.b"`, []string{"42"}},
		{`.["nope"]`, []string{"null"}},
		// Optional suffix.
		{".missing?", []string{"null"}},
		{".name?", []string{`"app"`}},
		{".server.host?", []string{`"localhost"`}},
		{".tags[0]?", []string{`"a"`}},
		{".tags[]?", []string{`"a"`, `"b"`, `"c"`}},
		// Comma (multiple outputs) and identity in a pipe.
		{".name, .port", []string{`"app"`, "8080"}},
		{".name | .", []string{`"app"`}},
		// Builtins over paths.
		{".tags | length", []string{"3"}},
		{"keys", []string{`[ "a.b", "name", "port", "server", "tags" ]`}},
		{".server | keys", []string{`[ "enabled", "host" ]`}},
		{"type", []string{`"object"`}},
		{".tags | type", []string{`"array"`}},
	}
	for _, tc := range cases {
		t.Run(tc.filter, func(t *testing.T) {
			got := evalToStrings(t, tc.filter, charInput)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("filter %q\n got: %#v\nwant: %#v", tc.filter, got, tc.want)
			}
		})
	}
}

// Identity returns the whole document (compact single-line form).
func TestFilterCharacterizationIdentity(t *testing.T) {
	got := evalToStrings(t, ".", "{ a = 1; b = \"x\" }")
	want := []string{`{ a = 1; b = "x" }`}
	if !reflect.DeepEqual(got, want) {
		t.Errorf(". identity\n got: %#v\nwant: %#v", got, want)
	}
}

// Iterating a non-array is an evaluation error, not a parse error.
func TestFilterCharacterizationIterateError(t *testing.T) {
	fil, err := parseFilter(".name[]")
	if err != nil {
		t.Fatalf("parseFilter: %v", err)
	}
	val, err := parseSetayToValue(charInput, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := evalFilter(fil, val, evalContext{}); err == nil || !strings.Contains(err.Error(), "cannot iterate") {
		t.Errorf("expected a \"cannot iterate\" eval error, got %v", err)
	}
}

// Slice syntax is not supported: it must fail at parse time.
func TestFilterCharacterizationSliceRejected(t *testing.T) {
	if _, err := parseFilter(".tags[1:2]"); err == nil {
		t.Error("expected .tags[1:2] to be a parse error")
	}
}
