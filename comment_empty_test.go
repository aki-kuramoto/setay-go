package setay_test

import (
	"testing"

	setay "github.com/aki-kuramoto/setay-go"
)

// A bare "#" immediately followed by a newline (LF, CRLF, or a lone CR) or by
// end-of-input is an empty single-line comment. This was previously a parse
// error; the grammar was loosened so agents can write comment-only "#" lines.
func TestEmptyHashComment(t *testing.T) {
	okCases := map[string]string{
		"lf":                 "{\n#\na = 1;\nb = 2;\n}",
		"crlf":               "{\r\n#\r\na = 1;\r\nb = 2;\r\n}",
		"lone-cr":            "{\r#\ra = 1;\r}",
		"hash-at-eof":        "{ a = 1; }\n#",
		"hash-at-eof-no-nl":  "{ a = 1; }#",
		"multiple-blank":     "{\n#\n#\n#\na = 1;\n}",
		"between-and-around": "#\n{\n#\na = 1;\n}\n#\n",
	}
	for name, input := range okCases {
		t.Run(name, func(t *testing.T) {
			if _, err := setay.Parse(input); err != nil {
				t.Fatalf("expected %q to parse, got error: %v", input, err)
			}
		})
	}
}

// "#" must still be followed by a recognized comment lead (" ", "\t", "#", "!",
// "{") or a newline/EOF. A "#" followed by any other character stays an error,
// so ordinary text is never silently swallowed as a comment.
func TestBareHashWithTextStillErrors(t *testing.T) {
	badCases := map[string]string{
		"hash-text":     "{\n#foo\na = 1;\n}",
		"hash-value":    "{\n#42\na = 1;\n}",
		"hash-then-eof": "{ a = 1; }\n#foo",
	}
	for name, input := range badCases {
		t.Run(name, func(t *testing.T) {
			if _, err := setay.Parse(input); err == nil {
				t.Fatalf("expected %q to be a parse error, but it parsed", input)
			}
		})
	}
}

// The values surrounding an empty "#" comment line must still unmarshal
// correctly (the "#" consumes only itself; the newline is normal spacing).
func TestEmptyHashCommentRoundTrip(t *testing.T) {
	type cfg struct {
		A int `setay:"a"`
		B int `setay:"b"`
	}
	input := []byte("{\n#\na = 1;\n#\nb = 2;\n#\n}")
	var got cfg
	if err := setay.Unmarshal(input, &got); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if got.A != 1 || got.B != 2 {
		t.Fatalf("expected {A:1 B:2}, got %+v", got)
	}
}
