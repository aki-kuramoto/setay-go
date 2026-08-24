package setay_test

import (
	"strings"
	"testing"

	setay "github.com/aki-kuramoto/setay-go"
)

// A flag entry is a dict entry written as a key alone, with "= value" omitted
// (e.g. { verbose; retries = 3; }). It denotes the boolean true: present = true,
// absent = false. These tests pin that behavior across unmarshal, the Document
// API, round-tripping, and the (deliberately lossy) marshal direction.

func TestFlagEntryUnmarshalStruct(t *testing.T) {
	type cfg struct {
		Verbose bool
		Debug   bool
		Quiet   bool
		Name    string
	}
	// verbose is a flag (true); debug is "= true" (also true); quiet is absent
	// (false). A flag and "= true" are two spellings of the same value.
	var c cfg
	src := `{ verbose; debug = true; name = "app"; }`
	if err := setay.Unmarshal([]byte(src), &c); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !c.Verbose {
		t.Error("verbose (flag entry) should decode to true")
	}
	if !c.Debug {
		t.Error("debug (= true) should decode to true")
	}
	if c.Quiet {
		t.Error("quiet (absent) should decode to false")
	}
	if c.Name != "app" {
		t.Errorf("name = %q, want app", c.Name)
	}
}

func TestFlagEntryUnmarshalNonBoolField(t *testing.T) {
	// A flag entry denotes true; targeting a non-bool field is an error.
	type cfg struct {
		Name string
	}
	var c cfg
	if err := setay.Unmarshal([]byte(`{ name; }`), &c); err == nil {
		t.Error("a flag entry into a string field should error")
	}
}

func TestFlagEntryUnmarshalMap(t *testing.T) {
	// map[string]bool: flag -> true, alongside an explicit false.
	var mb map[string]bool
	if err := setay.Unmarshal([]byte(`{ a; b = false; }`), &mb); err != nil {
		t.Fatalf("Unmarshal map[string]bool: %v", err)
	}
	if !mb["a"] || mb["b"] {
		t.Errorf("map[string]bool = %v, want a:true b:false", mb)
	}

	// map[string]any: flag -> the bool true.
	var ma map[string]any
	if err := setay.Unmarshal([]byte(`{ a; }`), &ma); err != nil {
		t.Fatalf("Unmarshal map[string]any: %v", err)
	}
	if v, ok := ma["a"].(bool); !ok || !v {
		t.Errorf("map[string]any[a] = %#v, want bool true", ma["a"])
	}

	// map[string]int: a flag entry (true) cannot fit; error.
	var mi map[string]int
	if err := setay.Unmarshal([]byte(`{ a; }`), &mi); err == nil {
		t.Error("a flag entry into map[string]int should error")
	}
}

func TestFlagEntryDocumentNode(t *testing.T) {
	doc, err := setay.ParseDocument("{ verbose; port = 8080; }\n")
	if err != nil {
		t.Fatal(err)
	}
	n, ok := doc.Get(":/verbose")
	if !ok {
		t.Fatal("Get(:/verbose) not found")
	}
	if n.Kind() != "bool" {
		t.Errorf("Kind() = %q, want bool", n.Kind())
	}
	// A flag entry has no value source text.
	if n.Raw() != "" {
		t.Errorf("Raw() = %q, want empty", n.Raw())
	}
	// But its value reads as true, so a struct-mapper is not surprised by a nil.
	var b bool
	if err := n.Unmarshal(&b); err != nil {
		t.Fatalf("Node.Unmarshal: %v", err)
	}
	if !b {
		t.Error("a flag entry's value should Unmarshal to true")
	}
	// It is not a container.
	if _, ok := n.Field("x"); ok {
		t.Error("Field on a flag entry should fail")
	}
	if _, ok := n.Index(0); ok {
		t.Error("Index on a flag entry should fail")
	}
}

func TestFlagEntryRoundTrip(t *testing.T) {
	corpus := []string{
		"{ verbose; retries = 3; }\n",
		"{\n\tverbose;\n\tdebug = true;\n\tlast\n}\n",
		"{ a; b; c }",
		"{\n\twhile-holding = \"xfer\";\n\tany-key;\n}\n",
	}
	for _, src := range corpus {
		doc, err := setay.ParseDocument(src)
		if err != nil {
			t.Errorf("ParseDocument(%q): %v", src, err)
			continue
		}
		if got := doc.String(); got != src {
			t.Errorf("round-trip mismatch\n in: %q\nout: %q", src, got)
		}
	}
}

func TestFlagEntryDelete(t *testing.T) {
	src := "{\n\tverbose;\n\tport = 8080;\n}\n"
	doc, err := setay.ParseDocument(src)
	if err != nil {
		t.Fatal(err)
	}
	cs := doc.Changes()
	n, ok := doc.Get(":/verbose")
	if !ok {
		t.Fatal("no :/verbose")
	}
	if err := cs.Delete(n); err != nil {
		t.Fatalf("Delete flag entry: %v", err)
	}
	nd, err := cs.Apply()
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	want := strings.Replace(src, "\tverbose;\n", "", 1)
	if got := nd.String(); got != want {
		t.Errorf("after delete\n got: %q\nwant: %q", got, want)
	}
}

func TestFlagEntryAppendAfterFlag(t *testing.T) {
	// AppendEntry must handle a dict whose last entry is a flag entry (no value
	// node to measure from): it appends after the flag's key line.
	src := "{\n\tport = 8080;\n\tverbose;\n}\n"
	doc, err := setay.ParseDocument(src)
	if err != nil {
		t.Fatal(err)
	}
	cs := doc.Changes()
	if err := cs.AppendEntry(doc.Root(), "debug", "true"); err != nil {
		t.Fatalf("AppendEntry after a flag entry: %v", err)
	}
	nd, err := cs.Apply()
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	want := "{\n\tport = 8080;\n\tverbose;\n\tdebug = true;\n}\n"
	if got := nd.String(); got != want {
		t.Errorf("after append\n got: %q\nwant: %q", got, want)
	}
}

func TestFlagEntrySetRawUnsupported(t *testing.T) {
	doc, err := setay.ParseDocument("{ verbose; }\n")
	if err != nil {
		t.Fatal(err)
	}
	cs := doc.Changes()
	n, _ := doc.Get(":/verbose")
	if err := cs.SetRaw(n, "false"); err == nil {
		t.Error("SetRaw on a flag entry should be unsupported (error)")
	}
}

func TestFlagEntryMarshalIsLossy(t *testing.T) {
	// Marshal never emits a flag entry: a bool true is always written as
	// "= true". The flag spelling exists only in hand-written / round-tripped
	// documents.
	type cfg struct {
		Verbose bool `setay:"verbose"`
	}
	out, err := setay.Marshal(cfg{Verbose: true})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "verbose = true") {
		t.Errorf("Marshal output %q should contain \"verbose = true\"", s)
	}
}
