package setaypath

import "testing"

// TestPathGrammarAcceptReject locks the boompaw-generated setay path grammar's
// shape: which paths it accepts and which it rejects. Decoding of the parsed
// segments is covered end-to-end by the Document tests in the parent package.
func TestPathGrammarAcceptReject(t *testing.T) {
	accept := []string{
		":/name",
		":/server.host",
		":/servers.0",
		":/ports.-1",
		":/a.b.c",
		":/with-hyphen",
		":/n123",
		":/_under",
		":/a.-1.b",       // negative index mid-path
		`:/"a.b"`,        // DQ quoted key (dot inside)
		`:/'a b'`,        // SQ quoted key (space inside)
		`:/'a${b}'`,      // SQ allows bare ${ }
		`:/x."q y".0`,    // mixed segments
		`:/"with \"x\""`, // DQ with an escaped quote
		`:/'\''`,         // SQ with an escaped quote
	}
	for _, s := range accept {
		if _, err := Parse(s); err != nil {
			t.Errorf("expected to accept %q, got error: %v", s, err)
		}
	}

	reject := []string{
		"",          // empty
		":/",        // root only, no segment
		"name",      // must start with ":/"
		": /a",      // ":/" is a single token, no interior space
		":/a.",      // trailing separator
		":/a..b",    // empty segment
		":/1abc",    // a segment starting with a digit is an index, "abc" is leftover
		":/-",       // an index needs at least one digit
		":/my-",     // unquoted key cannot end with a hyphen
		":/a b",     // no whitespace inside a path
		":/a[0]",    // brackets are not the index syntax here
		`:/"a${b}"`, // DQ reserves $ { } (must be escaped)
		`:/"unterm`, // unterminated quoted key
		`:/"\z"`,    // unknown escape in a quoted key
	}
	for _, s := range reject {
		if _, err := Parse(s); err == nil {
			t.Errorf("expected to reject %q, but it parsed", s)
		}
	}
}
