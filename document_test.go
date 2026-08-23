package setay_test

import (
	"strings"
	"testing"

	setay "github.com/aki-kuramoto/setay-go"
)

// A representative document with comments, tabs, hex, nested dict, and a list.
const editSample = "\uFEFF# header comment\n" +
	"{\n" +
	"\tname = \"app\";\n" +
	"\tport = 0xE;  # a hex port\n" +
	"\n" +
	"\tserver =\n" +
	"\t{\n" +
	"\t\thost = \"localhost\";\n" +
	"\t\tflags = [ \"x\", \"y\", \"z\" ];\n" +
	"\t};\n" +
	"\t\"a.b\" = 1;\n" +
	"}\n"

// ParseDocument then String must reproduce the input byte for byte, including
// BOM, comments, blank lines, and tabs.
func TestDocumentRoundTrip(t *testing.T) {
	corpus := []string{
		editSample,
		"{}\n",
		"{ a = 1; b = 2 }",
		"\uFEFF{\r\n\ta = 1;\r\n}\r\n",
		"# only a header\n{\n\t#\n\tx = [1, 2, 3];\n}\n",
	}
	for _, src := range corpus {
		doc, err := setay.ParseDocument(src)
		if err != nil {
			t.Fatalf("ParseDocument(%q): %v", src, err)
		}
		if got := doc.String(); got != src {
			t.Errorf("round-trip mismatch\n--- in  ---\n%q\n--- out ---\n%q", src, got)
		}
	}
}

func TestDocumentGetRaw(t *testing.T) {
	doc, err := setay.ParseDocument(editSample)
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct{ raw, kind string }{
		":/name":            {`"app"`, "string"},
		":/port":            {"0xE", "number"},
		":/server.host":     {`"localhost"`, "string"},
		":/server.flags":    {`[ "x", "y", "z" ]`, "list"},
		":/server.flags.0":  {`"x"`, "string"},
		":/server.flags.-1": {`"z"`, "string"},
		`:/"a.b"`:           {"1", "number"},
		`:/'a.b'`:           {"1", "number"}, // SQ quoted key reaches the same key
	}
	for path, want := range cases {
		n, ok := doc.Get(path)
		if !ok {
			t.Errorf("Get(%q): not found", path)
			continue
		}
		if n.Raw() != want.raw {
			t.Errorf("Get(%q).Raw() = %q, want %q", path, n.Raw(), want.raw)
		}
		if n.Kind() != want.kind {
			t.Errorf("Get(%q).Kind() = %q, want %q", path, n.Kind(), want.kind)
		}
	}
}

// Editing one value must change only that value's span; everything else stays
// byte-identical. Apply yields a new Document; the original is untouched.
func TestChangeSetSetRawSingle(t *testing.T) {
	doc, err := setay.ParseDocument(editSample)
	if err != nil {
		t.Fatal(err)
	}
	cs := doc.Changes()
	n, ok := doc.Get(":/port")
	if !ok {
		t.Fatal("no :/port")
	}
	if err := cs.SetRaw(n, "0x10"); err != nil {
		t.Fatalf("SetRaw: %v", err)
	}
	newDoc, err := cs.Apply()
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if doc.String() != editSample {
		t.Error("the original Document was mutated by Apply")
	}
	want := strings.Replace(editSample, "0xE", "0x10", 1)
	if got := newDoc.String(); got != want {
		t.Errorf("after edit\n--- got  ---\n%q\n--- want ---\n%q", got, want)
	}
}

func TestChangeSetNestedAndList(t *testing.T) {
	doc, err := setay.ParseDocument(editSample)
	if err != nil {
		t.Fatal(err)
	}
	host, _ := doc.Get(":/server.host")
	flag, _ := doc.Get(":/server.flags.1")
	cs := doc.Changes()
	if err := cs.SetRaw(host, `"127.0.0.1"`); err != nil {
		t.Fatalf("SetRaw host: %v", err)
	}
	if err := cs.SetRaw(flag, `"Y"`); err != nil {
		t.Fatalf("SetRaw flag: %v", err)
	}
	// Apply re-parses, so it also validates the result.
	newDoc, err := cs.Apply()
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	want := editSample
	want = strings.Replace(want, `"localhost"`, `"127.0.0.1"`, 1)
	want = strings.Replace(want, `"y"`, `"Y"`, 1)
	if got := newDoc.String(); got != want {
		t.Errorf("after edits\n--- got  ---\n%q\n--- want ---\n%q", got, want)
	}
}

// The low-level Node API reaches the same value; the ChangeSet edits it.
func TestDocumentNodeLowLevel(t *testing.T) {
	doc, err := setay.ParseDocument(editSample)
	if err != nil {
		t.Fatal(err)
	}
	server, ok := doc.Field("server")
	if !ok {
		t.Fatal("Field(server) not found")
	}
	host, ok := server.Field("host")
	if !ok {
		t.Fatal("server.Field(host) not found")
	}
	cs := doc.Changes()
	if err := cs.SetRaw(host, `"h2"`); err != nil {
		t.Fatalf("SetRaw: %v", err)
	}
	newDoc, err := cs.Apply()
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	want := strings.Replace(editSample, `"localhost"`, `"h2"`, 1)
	if got := newDoc.String(); got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestChangeSetOverlapRejected(t *testing.T) {
	doc, err := setay.ParseDocument(editSample)
	if err != nil {
		t.Fatal(err)
	}
	cs := doc.Changes()
	server, _ := doc.Get(":/server")
	if err := cs.SetRaw(server, "{ host = \"z\"; }"); err != nil {
		t.Fatalf("SetRaw server: %v", err)
	}
	// Editing something inside the already-replaced server span must be rejected
	// within the same ChangeSet.
	host, _ := doc.Get(":/server.host")
	if err := cs.SetRaw(host, `"nope"`); err == nil {
		t.Error("expected overlap error, got nil")
	}
}

// A node from a different document is rejected, and an edit that produces
// invalid setay is reported by Apply (which re-parses).
func TestChangeSetGuards(t *testing.T) {
	doc, _ := setay.ParseDocument(editSample)
	other, _ := setay.ParseDocument("{ a = 1; }")

	cs := doc.Changes()
	foreign, _ := other.Get(":/a")
	if err := cs.SetRaw(foreign, "2"); err == nil {
		t.Error("SetRaw with a node from another document should error")
	}

	cs2 := doc.Changes()
	n, _ := doc.Get(":/port")
	if err := cs2.SetRaw(n, "] not valid"); err != nil {
		t.Fatalf("SetRaw records verbatim: %v", err)
	}
	if _, err := cs2.Apply(); err == nil {
		t.Error("Apply should report the invalid spliced text as a parse error")
	}
}

func TestDocumentNodeUnmarshal(t *testing.T) {
	doc, err := setay.ParseDocument(editSample)
	if err != nil {
		t.Fatal(err)
	}
	n, ok := doc.Get(":/port")
	if !ok {
		t.Fatal("no :/port")
	}
	var port int
	if err := n.Unmarshal(&port); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if port != 0xE {
		t.Errorf("port = %d, want 14", port)
	}

	flags, ok := doc.Get(":/server.flags")
	if !ok {
		t.Fatal("no flags")
	}
	var xs []string
	if err := flags.Unmarshal(&xs); err != nil {
		t.Fatalf("Unmarshal flags: %v", err)
	}
	if strings.Join(xs, ",") != "x,y,z" {
		t.Errorf("flags = %v", xs)
	}
}

func TestDocumentGetErrors(t *testing.T) {
	doc, err := setay.ParseDocument(editSample)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{":/missing", ":/server.nope", ":/server.flags.9", ":/server.flags.", "name", ""} {
		if _, ok := doc.Get(p); ok {
			t.Errorf("Get(%q) unexpectedly succeeded", p)
		}
	}
}
