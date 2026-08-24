package setay_test

import (
	"strings"
	"testing"

	setay "github.com/aki-kuramoto/setay-go"
)

// An unterminated string reports a clear message naming the quote style and the
// position it opened at, rather than the raw parser's "expected \"\\\", got EOF".
func TestUnterminatedStringError(t *testing.T) {
	cases := []struct {
		name, doc, want string
	}{
		{"sq", `{ s = 'abc }`, "unterminated single-quoted string"},
		{"dq", `{ s = "abc }`, "unterminated double-quoted string"},
	}
	for _, tc := range cases {
		var v map[string]any
		err := setay.Unmarshal([]byte(tc.doc), &v)
		if err == nil {
			t.Errorf("%s: expected an error", tc.name)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error %q does not contain %q", tc.name, err.Error(), tc.want)
		}
	}

	// The opening position is reported (here the ' is on line 2).
	err := setay.Unmarshal([]byte("{\n\ts = 'abc\n}"), new(map[string]any))
	if err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Errorf("expected the opening line in the message, got: %v", err)
	}

	// ParseDocument surfaces the same improved message.
	if _, err := setay.ParseDocument(`{ s = 'abc }`); err == nil ||
		!strings.Contains(err.Error(), "unterminated single-quoted string") {
		t.Errorf("ParseDocument: got %v", err)
	}
}

// A quote that sits inside a comment (not an open string) must not be misreported
// as an unterminated string; such documents parse cleanly.
func TestUnterminatedStringNoFalsePositive(t *testing.T) {
	ok := []string{
		"{\n\t# it's fine\n\ta = 1;\n}",   // apostrophe in a line comment
		"{\n\t#{ a ' b }#\n\tx = 1;\n}",   // quote in a block comment
		`{ a = "he said \"hi\""; b = 1 }`, // escaped quotes, all closed
	}
	for _, doc := range ok {
		if err := setay.Unmarshal([]byte(doc), new(map[string]any)); err != nil {
			t.Errorf("%q should parse, got: %v", doc, err)
		}
	}
}
