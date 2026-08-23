package setay_test

import (
	"strings"
	"testing"

	setay "github.com/aki-kuramoto/setay-go"
)

const editDoc = "{\n" +
	"\tname = \"app\";\n" +
	"\tport = 8080;\n" +
	"\ttags = [\n" +
	"\t\t\"a\",\n" +
	"\t\t\"b\",\n" +
	"\t];\n" +
	"}\n"

const nestedDoc = "{\n" +
	"\tserver = {\n" +
	"\t\thost = \"h\";\n" +
	"\t\tport = 1;\n" +
	"\t};\n" +
	"}\n"

// applyOne parses src, lets build record changes, applies them, checks that the
// original document is unchanged, and returns the new document's text.
func applyOne(t *testing.T, src string, build func(cs *setay.ChangeSet, doc *setay.Document) error) string {
	t.Helper()
	doc, err := setay.ParseDocument(src)
	if err != nil {
		t.Fatal(err)
	}
	cs := doc.Changes()
	if err := build(cs, doc); err != nil {
		t.Fatalf("recording changes: %v", err)
	}
	nd, err := cs.Apply()
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if doc.String() != src {
		t.Error("the original Document was mutated")
	}
	return nd.String()
}

func TestEditRename(t *testing.T) {
	got := applyOne(t, editDoc, func(cs *setay.ChangeSet, doc *setay.Document) error {
		n, _ := doc.Get(":/port")
		return cs.Rename(n, "PORT")
	})
	want := strings.Replace(editDoc, "port =", "PORT =", 1)
	if got != want {
		t.Errorf("got\n%q\nwant\n%q", got, want)
	}
}

func TestEditDelete(t *testing.T) {
	cases := []struct {
		name, path, removed string
	}{
		{"middle", ":/port", "\tport = 8080;\n"},
		{"first", ":/name", "\tname = \"app\";\n"},
		{"last-multiline", ":/tags", "\ttags = [\n\t\t\"a\",\n\t\t\"b\",\n\t];\n"},
		{"list-elem", ":/tags.0", "\t\t\"a\",\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := applyOne(t, editDoc, func(cs *setay.ChangeSet, doc *setay.Document) error {
				n, ok := doc.Get(tc.path)
				if !ok {
					t.Fatalf("no %s", tc.path)
				}
				return cs.Delete(n)
			})
			want := strings.Replace(editDoc, tc.removed, "", 1)
			if got != want {
				t.Errorf("got\n%q\nwant\n%q", got, want)
			}
		})
	}
}

func TestEditAppendEntryTopLevel(t *testing.T) {
	got := applyOne(t, editDoc, func(cs *setay.ChangeSet, doc *setay.Document) error {
		return cs.AppendEntry(doc.Root(), "debug", "true")
	})
	want := strings.Replace(editDoc, "\t];\n}\n", "\t];\n\tdebug = true;\n}\n", 1)
	if got != want {
		t.Errorf("got\n%q\nwant\n%q", got, want)
	}
}

func TestEditAppendEntryNested(t *testing.T) {
	got := applyOne(t, nestedDoc, func(cs *setay.ChangeSet, doc *setay.Document) error {
		n, _ := doc.Get(":/server")
		return cs.AppendEntry(n, "debug", "true")
	})
	want := strings.Replace(nestedDoc, "\t\tport = 1;\n\t}", "\t\tport = 1;\n\t\tdebug = true;\n\t}", 1)
	if got != want {
		t.Errorf("got\n%q\nwant\n%q", got, want)
	}
}

func TestEditPrependEntry(t *testing.T) {
	got := applyOne(t, editDoc, func(cs *setay.ChangeSet, doc *setay.Document) error {
		return cs.PrependEntry(doc.Root(), "first", "0")
	})
	want := strings.Replace(editDoc, "{\n\tname", "{\n\tfirst = 0;\n\tname", 1)
	if got != want {
		t.Errorf("got\n%q\nwant\n%q", got, want)
	}
}

func TestEditAppendElem(t *testing.T) {
	got := applyOne(t, editDoc, func(cs *setay.ChangeSet, doc *setay.Document) error {
		n, _ := doc.Get(":/tags")
		return cs.AppendElem(n, `"c"`)
	})
	want := strings.Replace(editDoc, "\t\t\"b\",\n\t]", "\t\t\"b\",\n\t\t\"c\",\n\t]", 1)
	if got != want {
		t.Errorf("got\n%q\nwant\n%q", got, want)
	}
}

func TestEditPrependElem(t *testing.T) {
	got := applyOne(t, editDoc, func(cs *setay.ChangeSet, doc *setay.Document) error {
		n, _ := doc.Get(":/tags")
		return cs.PrependElem(n, `"z"`)
	})
	want := strings.Replace(editDoc, "[\n\t\t\"a\"", "[\n\t\t\"z\",\n\t\t\"a\"", 1)
	if got != want {
		t.Errorf("got\n%q\nwant\n%q", got, want)
	}
}

func TestEditAppendRaw(t *testing.T) {
	got := applyOne(t, editDoc, func(cs *setay.ChangeSet, doc *setay.Document) error {
		return cs.AppendRaw(doc.Root(), "\tx = 1;\n")
	})
	want := strings.Replace(editDoc, "\t];\n}\n", "\t];\n\tx = 1;\n}\n", 1)
	if got != want {
		t.Errorf("got\n%q\nwant\n%q", got, want)
	}
}

// Combining several changes in one ChangeSet applies them together.
func TestEditCombined(t *testing.T) {
	got := applyOne(t, editDoc, func(cs *setay.ChangeSet, doc *setay.Document) error {
		port, _ := doc.Get(":/port")
		if err := cs.SetRaw(port, "9090"); err != nil {
			return err
		}
		name, _ := doc.Get(":/name")
		if err := cs.Rename(name, "title"); err != nil {
			return err
		}
		return cs.AppendEntry(doc.Root(), "debug", "true")
	})
	want := editDoc
	want = strings.Replace(want, "8080", "9090", 1)
	want = strings.Replace(want, "name =", "title =", 1)
	want = strings.Replace(want, "\t];\n}\n", "\t];\n\tdebug = true;\n}\n", 1)
	if got != want {
		t.Errorf("got\n%q\nwant\n%q", got, want)
	}
}

func TestEditGuards(t *testing.T) {
	doc, _ := setay.ParseDocument(editDoc)
	cs := doc.Changes()

	name, _ := doc.Get(":/name") // a string value, not a dict/entry container
	if err := cs.AppendEntry(name, "k", "v"); err == nil {
		t.Error("AppendEntry on a non-dict should error")
	}
	if err := cs.AppendElem(name, "1"); err == nil {
		t.Error("AppendElem on a non-list should error")
	}
	if err := cs.Rename(doc.Root(), "x"); err == nil {
		t.Error("Rename on the root (no key) should error")
	}
	if err := cs.Delete(doc.Root()); err == nil {
		t.Error("Delete on the root should error")
	}
}
