package setay_test

import (
	"testing"

	setay "github.com/aki-kuramoto/setay-go"
)

// Marshal emits single-quoted strings, which is setay's stable, non-interpolating
// string form. As a result, values that a double-quoted string would reject or
// misread — bare $ { } (reserved in DQ) and a literal ${...} (which DQ would
// interpolate) — round-trip through Marshal → Unmarshal unchanged.
func TestMarshalRoundtripSpecials(t *testing.T) {
	type S struct {
		V string `setay:"v"`
	}
	values := []string{
		"price $100",
		"a {b} c",
		"literal ${HOME} is not interpolated",
		`he said "hi"`,
		"it's a 'quoted' word",
		"tab\tand\nnewline\r0\x00end",
		"back\\slash",
		"plain",
		"everything $ { } \" ' \\ at once",
		"unicode: Aあ😀",
	}
	for _, in := range values {
		out, err := setay.Marshal(S{V: in})
		if err != nil {
			t.Errorf("Marshal(%q): %v", in, err)
			continue
		}
		var back S
		if err := setay.Unmarshal(out, &back); err != nil {
			t.Errorf("Unmarshal(Marshal(%q)) failed: %v\n  output: %s", in, err, out)
			continue
		}
		if back.V != in {
			t.Errorf("round-trip mismatch:\n  in:   %q\n  back: %q\n  output: %s", in, back.V, out)
		}
	}
}
