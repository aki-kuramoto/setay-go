package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFormatGolden formats every testdata/fmt/*.in.setay file and asserts the
// result matches its hand-written testdata/fmt/*.want.setay counterpart, byte
// for byte.
//
// ┌─────────────────────────────────────────────────────────────────────────┐
// │ DO NOT generate the *.want.setay files by running `setay fmt` (or        │
// │ Format) and saving the output. They MUST be written and edited BY HAND.  │
// │                                                                          │
// │ Why: the point of a golden test is to pin the *intended* canonical       │
// │ format as decided by a human. If the expected file were produced by the  │
// │ formatter itself, this test would only ever answer "does the formatter   │
// │ still emit what it emitted last time?" — it could never answer "is the   │
// │ output correct?". A real formatting bug would be captured into the       │
// │ golden and silently blessed.                                             │
// │                                                                          │
// │ So: when a fixture's output legitimately changes, look at the diff, make │
// │ a human judgement about the correct form, and type the new *.want.setay  │
// │ by hand. Never pipe fmt output into it.                                  │
// └─────────────────────────────────────────────────────────────────────────┘
func TestFormatGolden(t *testing.T) {
	ins, err := filepath.Glob("testdata/fmt/*.in.setay")
	if err != nil {
		t.Fatal(err)
	}
	if len(ins) == 0 {
		t.Fatal("no golden fixtures found under testdata/fmt/")
	}

	for _, in := range ins {
		name := strings.TrimSuffix(filepath.Base(in), ".in.setay")
		t.Run(name, func(t *testing.T) {
			srcBytes, err := os.ReadFile(in)
			if err != nil {
				t.Fatal(err)
			}
			wantPath := strings.TrimSuffix(in, ".in.setay") + ".want.setay"
			wantBytes, err := os.ReadFile(wantPath)
			if err != nil {
				t.Fatalf("missing golden %s (write it by hand — see file header): %v", wantPath, err)
			}

			source := string(srcBytes)
			doc, perr := Parse(source)
			if perr != nil {
				t.Fatalf("input did not parse: %v", perr)
			}
			got := Format(source, doc)

			if want := string(wantBytes); got != want {
				t.Errorf("formatted output does not match %s\n--- got ---\n%s\n--- want ---\n%s\n--- got (quoted) ---\n%q\n--- want (quoted) ---\n%q",
					filepath.Base(wantPath), got, want, got, want)
			}

			// The formatter must be idempotent: formatting already-formatted
			// output changes nothing. This guards against unstable output
			// independently of the hand-written goldens.
			doc2, perr := Parse(got)
			if perr != nil {
				t.Fatalf("formatted output did not re-parse: %v", perr)
			}
			if got2 := Format(got, doc2); got2 != got {
				t.Errorf("format is not idempotent for %s\n--- pass 1 ---\n%q\n--- pass 2 ---\n%q", name, got, got2)
			}
		})
	}
}
