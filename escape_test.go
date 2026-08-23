package setay_test

import (
	"testing"

	setay "github.com/aki-kuramoto/setay-go"
)

type escStr struct {
	S string `setay:"s"`
}

func decodeS(t *testing.T, doc string) (string, error) {
	t.Helper()
	var v escStr
	err := setay.Unmarshal([]byte(doc), &v)
	return v.S, err
}

// Valid escapes decode to the intended value. In a double-quoted string every
// ASCII symbol may be written with a leading backslash (an optional alternative
// form), and $ { } MUST be, and \xHH/\uXXXX/\UXXXXXXXX carry code points.
func TestEscapeValidDQ(t *testing.T) {
	cases := map[string]string{
		`{ s = "\!" }`:          "!",
		`{ s = "\." }`:          ".",
		`{ s = "\,\;\:\?" }`:    ",;:?",
		`{ s = "\$" }`:          "$",
		`{ s = "\{\}" }`:        "{}",
		`{ s = "\\" }`:          "\\",
		`{ s = "\"" }`:          "\"",
		`{ s = "a\?b\:c" }`:     "a?b:c",
		`{ s = "\x41\x7F" }`:    "A\x7f",
		`{ s = "Aあ" }`:          "Aあ",
		`{ s = "\U0001F600" }`:  "\U0001F600",
		`{ s = "\n\r\t\0" }`:    "\n\r\t\x00",
		`{ s = "price \$100" }`: "price $100",
	}
	for doc, want := range cases {
		got, err := decodeS(t, doc)
		if err != nil {
			t.Errorf("%s -> error: %v", doc, err)
			continue
		}
		if got != want {
			t.Errorf("%s -> %q, want %q", doc, got, want)
		}
	}
}

// Single-quoted strings use "minimal escaping": only ' (the delimiter) and \
// must be escaped. $ { } -- and " and every other symbol -- are written bare,
// and SQ does not interpolate. It is NOT raw: the full escape table still works.
func TestEscapeSQ(t *testing.T) {
	valid := map[string]string{
		`{ s = '${VAR} $100 {a} b}' }`: "${VAR} $100 {a} b}", // $ { } bare, no interpolation
		`{ s = 'a"b' }`:                `a"b`,                // bare " is fine in SQ
		`{ s = 'x\'y' }`:               "x'y",                // the delimiter must be escaped
		`{ s = '\$\{\}' }`:             "${}",                // escapes still allowed (optional here)
		`{ s = '\n\t\!\.' }`:           "\n\t!.",             // control + symbol escapes work
	}
	for doc, want := range valid {
		got, err := decodeS(t, doc)
		if err != nil {
			t.Errorf("%s -> error: %v", doc, err)
			continue
		}
		if got != want {
			t.Errorf("%s -> %q, want %q", doc, got, want)
		}
	}
	// An unknown letter escape is still an error, even in SQ.
	if _, err := decodeS(t, `{ s = 'a\b' }`); err == nil {
		t.Error(`SQ '\b' (unknown letter escape) should be a parse error`)
	}
}

// Reserved-bare and unknown-escape cases are parse errors.
func TestEscapeInvalidDQ(t *testing.T) {
	bad := []string{
		`{ s = "a $ b" }`, // bare '$' (not '${', not '\$')
		`{ s = "a { b" }`, // bare '{'
		`{ s = "a } b" }`, // bare '}'
		`{ s = "\z" }`,    // unknown letter escape (reserved)
		`{ s = "\9" }`,    // unknown digit escape (reserved)
		`{ s = "\x80" }`,  // \xHH out of range (> 0x7F)
		`{ s = "\xG0" }`,  // bad hex
		`{ s = "\u041" }`, // \u needs exactly 4 hex
	}
	for _, doc := range bad {
		if _, err := decodeS(t, doc); err == nil {
			t.Errorf("%s should be a parse error, but it parsed", doc)
		}
	}
}
