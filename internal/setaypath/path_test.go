package setaypath

import "testing"

// TestPathGrammarAcceptReject locks the boompaw-generated path grammar's shape:
// which paths it accepts and which it rejects. Decoding of the parsed steps is
// covered end-to-end by the Document tests in the parent package.
func TestPathGrammarAcceptReject(t *testing.T) {
	accept := []string{
		".name",
		".server.host",
		".servers[0]",
		".ports[-1]",
		`.["a.b"]`,
		`."quoted key"`,
		".a[0].b",
		".a.b.c",
		".with-hyphen",
		".n123",
		"._under",
		`.["with \"escape\""]`,
	}
	for _, s := range accept {
		if _, err := Parse(s); err != nil {
			t.Errorf("expected to accept %q, got error: %v", s, err)
		}
	}

	reject := []string{
		"",        // empty
		".",       // identity is not a single value step
		".[]",     // streaming iterate — belongs to setayq, not here
		"a.b",     // must start with '.'
		".a.",     // trailing dot with no field
		".[",      // unterminated bracket
		".1abc",   // field name cannot start with a digit
		".a b",    // no whitespace inside a path
		".a | .b", // pipes belong to setayq
		".a[x]",   // non-numeric, non-quoted index
	}
	for _, s := range reject {
		if _, err := Parse(s); err == nil {
			t.Errorf("expected to reject %q, but it parsed", s)
		}
	}
}
