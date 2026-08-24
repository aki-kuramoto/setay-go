package setay_test

import (
	"testing"

	setay "github.com/aki-kuramoto/setay-go"
)

// bs is a single backslash. The \u / \U escape sequences under test are built by
// concatenating bs with the rest of the sequence, so no literal "\u..." text
// appears in this source (which keeps tooling from rewriting it).
const bs = "\\"

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

// Single-quoted strings use "minimal REQUIRED escaping": the same escape table
// as DQ is available, but only ' (the delimiter) and \ MUST be escaped. $ { } --
// and " and every other symbol -- are written bare, and SQ does not interpolate.
// It is NOT raw, and not "fewer escapes" -- just fewer mandatory ones.
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

// A \u escape high surrogate combines with an immediately following \u low
// surrogate into one code point (UTF-16 style), for interoperability with data
// from UTF-16-era systems.
func TestEscapeSurrogatePair(t *testing.T) {
	cases := []struct {
		seq  string
		want string
	}{
		{bs + `uD83D` + bs + `uDE00`, string(rune(0x1F600))}, // 😀
		{bs + `uD834` + bs + `uDD1E`, string(rune(0x1D11E))}, // 𝄞 musical G clef
		{bs + `uD800` + bs + `uDC00`, string(rune(0x10000))}, // lowest astral code point
	}
	for _, c := range cases {
		doc := `{ s = "` + c.seq + `" }`
		got, err := decodeS(t, doc)
		if err != nil {
			t.Errorf("%s -> error: %v", doc, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s -> %q, want %q", doc, got, c.want)
		}
	}
}

// A surrogate that is not a valid, ordered \u pair is rejected: a lone surrogate,
// a high surrogate not followed by a low one, a low surrogate on its own, any \U
// surrogate, and any value above U+10FFFF. The grammar accepts the hex digits;
// the value is checked when decoding.
func TestEscapeInvalidCodePoint(t *testing.T) {
	strs := []string{
		`"` + bs + `uD800"`,                // lone high surrogate
		`"` + bs + `uDBFF"`,                // high surrogate (upper end)
		`"` + bs + `uDC00"`,                // lone low surrogate
		`"` + bs + `uDFFF"`,                // low surrogate (upper end)
		`"` + bs + `uD800` + bs + `u0041"`, // high surrogate followed by a non-low escape
		`"` + bs + `uD800X"`,               // high surrogate followed by a normal char
		`"` + bs + `uDC00` + bs + `uD800"`, // low then high (wrong order)
		`"` + bs + `U0000D800"`,            // surrogate via \U (never pairs)
		`"` + bs + `U00110000"`,            // one past U+10FFFF
		`"` + bs + `UFFFFFFFF"`,            // far out of range
		`'` + bs + `uDEAD'`,                // lone surrogate in a single-quoted string too
	}
	for _, str := range strs {
		doc := `{ s = ` + str + ` }`
		if got, err := decodeS(t, doc); err == nil {
			t.Errorf("%s should be an error, but decoded to %q", doc, got)
		}
	}
}

// Code points adjacent to the surrogate block, and the maximum code point,
// decode normally through both \u and \U.
func TestEscapeValidCodePoint(t *testing.T) {
	cases := []struct {
		seq  string
		want string
	}{
		{bs + `uD7FF`, string(rune(0xD7FF))},       // just below the surrogate block
		{bs + `uE000`, string(rune(0xE000))},       // just above it
		{bs + `uFFFF`, string(rune(0xFFFF))},       // BMP noncharacter, still a valid scalar
		{bs + `U0010FFFF`, string(rune(0x10FFFF))}, // the maximum code point
	}
	for _, c := range cases {
		doc := `{ s = "` + c.seq + `" }`
		got, err := decodeS(t, doc)
		if err != nil {
			t.Errorf("%s -> error: %v", doc, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s -> %q, want %q", doc, got, c.want)
		}
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
