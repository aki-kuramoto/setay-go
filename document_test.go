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
		".name":             {`"app"`, "string"},
		".port":             {"0xE", "number"},
		".server.host":      {`"localhost"`, "string"},
		".server.flags":     {`[ "x", "y", "z" ]`, "list"},
		".server.flags[0]":  {`"x"`, "string"},
		".server.flags[-1]": {`"z"`, "string"},
		`.["a.b"]`:          {"1", "number"},
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
// byte-identical. The oracle is a single targeted string replacement.
func TestDocumentSetRawSingle(t *testing.T) {
	doc, err := setay.ParseDocument(editSample)
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.SetRaw(".port", "0x10"); err != nil {
		t.Fatalf("SetRaw: %v", err)
	}
	want := strings.Replace(editSample, "0xE", "0x10", 1)
	if got := doc.String(); got != want {
		t.Errorf("after edit\n--- got  ---\n%q\n--- want ---\n%q", got, want)
	}
}

func TestDocumentSetRawNestedAndList(t *testing.T) {
	doc, err := setay.ParseDocument(editSample)
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.SetRaw(".server.host", `"127.0.0.1"`); err != nil {
		t.Fatalf("SetRaw host: %v", err)
	}
	if err := doc.SetRaw(".server.flags[1]", `"Y"`); err != nil {
		t.Fatalf("SetRaw flag: %v", err)
	}
	want := editSample
	want = strings.Replace(want, `"localhost"`, `"127.0.0.1"`, 1)
	want = strings.Replace(want, `"y"`, `"Y"`, 1)
	if got := doc.String(); got != want {
		t.Errorf("after edits\n--- got  ---\n%q\n--- want ---\n%q", got, want)
	}
	// The edited document must still parse.
	if err := doc.Validate(); err != nil {
		t.Errorf("Validate after edits: %v", err)
	}
}

// The low-level Node API reaches the same value and edits it in place.
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
	if err := host.SetRaw(`"h2"`); err != nil {
		t.Fatalf("SetRaw: %v", err)
	}
	want := strings.Replace(editSample, `"localhost"`, `"h2"`, 1)
	if got := doc.String(); got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestDocumentOverlapRejected(t *testing.T) {
	doc, err := setay.ParseDocument(editSample)
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.SetRaw(".server", "{ host = \"z\"; }"); err != nil {
		t.Fatalf("SetRaw server: %v", err)
	}
	// Editing something inside the already-replaced server span must be rejected.
	if err := doc.SetRaw(".server.host", `"nope"`); err == nil {
		t.Error("expected overlap error, got nil")
	}
}

func TestDocumentNodeUnmarshal(t *testing.T) {
	doc, err := setay.ParseDocument(editSample)
	if err != nil {
		t.Fatal(err)
	}
	n, ok := doc.Get(".port")
	if !ok {
		t.Fatal("no .port")
	}
	var port int
	if err := n.Unmarshal(&port); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if port != 0xE {
		t.Errorf("port = %d, want 14", port)
	}

	flags, ok := doc.Get(".server.flags")
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
	for _, p := range []string{".missing", ".server.nope", ".server.flags[9]", ".server.flags[]", "name", ""} {
		if _, ok := doc.Get(p); ok {
			t.Errorf("Get(%q) unexpectedly succeeded", p)
		}
	}
	if err := doc.SetRaw(".missing", "1"); err == nil {
		t.Error("SetRaw on missing path should error")
	}
}
