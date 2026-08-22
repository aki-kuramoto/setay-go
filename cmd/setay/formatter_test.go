package main

import (
	"testing"
)

// mustFormat parses and formats src, failing the test on a parse error.
func mustFormat(t *testing.T, src string) string {
	t.Helper()
	doc, err := Parse(src)
	if err != nil {
		t.Fatalf("parse error for %q: %v", src, err)
	}
	return Format(src, doc)
}

// TestFormatCommentsAndSpacing pins the exact formatted output for small,
// targeted inputs. Each `want` is written by hand (never produced by running
// the formatter) — see formatter_golden_test.go for the rationale.
func TestFormatCommentsAndSpacing(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			// Comment in the leading position (right after '{') used to be
			// dropped for every comment form. All forms must survive.
			name: "leading_all_forms",
			in:   "{\n# a\n## b\n#\tc\n#! d\n#\nx = 1;\n}\n",
			want: "{\n\t# a\n\t## b\n\t#\tc\n\t#! d\n\t#\n\tx = 1;\n}\n",
		},
		{
			name: "empty_hash_leading",
			in:   "{\n#\nx = 1;\n}\n",
			want: "{\n\t#\n\tx = 1;\n}\n",
		},
		{
			// Empty '#' between entries: dropped before because parseSpacingInfo
			// did not recognize '#' followed by a newline.
			name: "empty_hash_between",
			in:   "{\na = 1;\n#\nb = 2;\n}\n",
			want: "{\n\ta = 1;\n\t#\n\tb = 2;\n}\n",
		},
		{
			name: "between_comment",
			in:   "{\na = 1;\n# mid\nb = 2;\n}\n",
			want: "{\n\ta = 1;\n\t# mid\n\tb = 2;\n}\n",
		},
		{
			name: "trailing_comment",
			in:   "{\na = 1;\n# bye\n}\n",
			want: "{\n\ta = 1;\n\t# bye\n}\n",
		},
		{
			// Leading comment inside a nested dict.
			name: "nested_leading",
			in:   "{\no = {\n# c\nx = 1;\n};\n}\n",
			want: "{\n\to =\n\t{\n\t\t# c\n\t\tx = 1;\n\t};\n}\n",
		},
		{
			// Multi-line comment in leading position is re-indented, not dropped.
			name: "multiline_comment_leading",
			in:   "{\n#{ a\nb }#\nx = 1;\n}\n",
			want: "{\n\t#{ a\n\tb }#\n\tx = 1;\n}\n",
		},
		{
			// A short, comment-free list stays on one line.
			name: "short_list_single_line",
			in:   "{\nv = [1,2,3];\n}\n",
			want: "{\n\tv = [ 1, 2, 3 ];\n}\n",
		},
		{
			// A multi-line list opens Allman-style, sharing the '{' rule: the
			// '[' goes on its own line beneath the key.
			name: "list_multiline_allman",
			in:   "{\nv = [\n# c\n1,\n2,\n];\n}\n",
			want: "{\n\tv =\n\t[\n\t\t# c\n\t\t1,\n\t\t2,\n\t];\n}\n",
		},
		{
			// The author's single-line list is kept single-line (Setay does not
			// force multi-line); inline spacing is normalized.
			name: "list_singleline_kept",
			in:   "{\nv = [1,2,3];\n}\n",
			want: "{\n\tv = [ 1, 2, 3 ];\n}\n",
		},
		{
			// A single-line list containing nested lists stays single-line when
			// the author wrote it that way.
			name: "nested_list_singleline_kept",
			in:   "{\nv = [[1,2],[3,4]];\n}\n",
			want: "{\n\tv = [ [ 1, 2 ], [ 3, 4 ] ];\n}\n",
		},
		{
			// A single-line dict is kept single-line: "{ k = v; k = v }".
			name: "dict_singleline_kept",
			in:   "{\nx = { a = 1; b = 2 };\n}\n",
			want: "{\n\tx = { a = 1; b = 2 };\n}\n",
		},
		{
			// An empty dict written inline stays inline.
			name: "empty_dict_inline_kept",
			in:   "{\nx = {};\n}\n",
			want: "{\n\tx = {};\n}\n",
		},
		{
			// An empty dict the author split open is preserved (content is about
			// to be added; keeping it split keeps the eventual diff clean).
			name: "empty_dict_split_kept",
			in:   "{\nx = {\n};\n}\n",
			want: "{\n\tx =\n\t{\n\t};\n}\n",
		},
		{
			// A blank line keeps the indentation of its depth — this is the
			// established setay formatter behavior.
			name: "blank_line_keeps_indent",
			in:   "{\na = 1;\n\nb = 2;\n}\n",
			want: "{\n\ta = 1;\n\t\n\tb = 2;\n}\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mustFormat(t, tc.in)
			if got != tc.want {
				t.Errorf("mismatch\n--- got  ---\n%q\n--- want ---\n%q", got, tc.want)
			}
		})
	}
}
