package setay_test

import (
	"errors"
	"reflect"
	"testing"

	setay "github.com/aki-kuramoto/setay-go"
)

type ufTLS struct {
	Enabled bool `setay:"enabled"`
}
type ufServer struct {
	Host string `setay:"host"`
	TLS  ufTLS  `setay:"tls"`
}
type ufConfig struct {
	Name   string   `setay:"name"`
	Server ufServer `setay:"server"`
}

// By default (no option), unknown keys are ignored, matching encoding/json.
func TestUnknownFieldsIgnoredByDefault(t *testing.T) {
	src := `{ name = "app"; bogus = 1; }`
	var c ufConfig
	if err := setay.Unmarshal([]byte(src), &c); err != nil {
		t.Fatalf("default Unmarshal should ignore unknown keys, got: %v", err)
	}
	if c.Name != "app" {
		t.Errorf("name = %q, want app", c.Name)
	}
}

// With DisallowUnknownFields and no unknown keys, there is no error.
func TestUnknownFieldsStrictClean(t *testing.T) {
	src := `{ name = "app"; server = { host = "h"; tls = { enabled = true; }; }; }`
	var c ufConfig
	if err := setay.Unmarshal([]byte(src), &c, setay.DisallowUnknownFields()); err != nil {
		t.Fatalf("clean document should not error under strict mode, got: %v", err)
	}
}

// Strict mode collects every unknown key across the whole document, as dotted
// paths from the root, in document order.
func TestUnknownFieldsStrictCollectsAll(t *testing.T) {
	src := `{
		name = "app";
		bogus = 1;
		server = {
			host = "h";
			tls = { enabled = true; oops = 2; };
			stray = 3;
		};
	}`
	var c ufConfig
	err := setay.Unmarshal([]byte(src), &c, setay.DisallowUnknownFields())
	if err == nil {
		t.Fatal("expected an error for unknown keys")
	}
	var ufe *setay.UnknownFieldsError
	if !errors.As(err, &ufe) {
		t.Fatalf("error is not *UnknownFieldsError: %T (%v)", err, err)
	}
	want := []string{"bogus", "server.tls.oops", "server.stray"}
	if !reflect.DeepEqual(ufe.Keys, want) {
		t.Errorf("Keys = %v, want %v", ufe.Keys, want)
	}
	// The values are still decoded; strict only adds the reported error.
	if c.Name != "app" || c.Server.Host != "h" || !c.Server.TLS.Enabled {
		t.Errorf("known fields should still decode: %+v", c)
	}
}

// A single unknown key produces a singular message.
func TestUnknownFieldsErrorMessage(t *testing.T) {
	var c ufConfig
	err := setay.Unmarshal([]byte(`{ name = "a"; bogus = 1; }`), &c, setay.DisallowUnknownFields())
	if err == nil || err.Error() != `setay: unknown field "bogus"` {
		t.Fatalf("got %v", err)
	}
}

// Unknown keys inside a struct element of a slice get an index in their path.
func TestUnknownFieldsInSlice(t *testing.T) {
	type item struct {
		ID int `setay:"id"`
	}
	type doc struct {
		Items []item `setay:"items"`
	}
	src := `{ items = [ { id = 1; x = 2; }, { id = 2; }, ]; }`
	var d doc
	err := setay.Unmarshal([]byte(src), &d, setay.DisallowUnknownFields())
	var ufe *setay.UnknownFieldsError
	if !errors.As(err, &ufe) {
		t.Fatalf("expected *UnknownFieldsError, got %T (%v)", err, err)
	}
	if !reflect.DeepEqual(ufe.Keys, []string{"items.0.x"}) {
		t.Errorf("Keys = %v, want [items.0.x]", ufe.Keys)
	}
}

// Unknown keys inside a struct-valued map get the map key in their path.
func TestUnknownFieldsInMapValue(t *testing.T) {
	type srv struct {
		Host string `setay:"host"`
	}
	type doc struct {
		Servers map[string]srv `setay:"servers"`
	}
	src := `{ servers = { web = { host = "h"; extra = 1; }; }; }`
	var d doc
	err := setay.Unmarshal([]byte(src), &d, setay.DisallowUnknownFields())
	var ufe *setay.UnknownFieldsError
	if !errors.As(err, &ufe) {
		t.Fatalf("expected *UnknownFieldsError, got %T (%v)", err, err)
	}
	if !reflect.DeepEqual(ufe.Keys, []string{"servers.web.extra"}) {
		t.Errorf("Keys = %v, want [servers.web.extra]", ufe.Keys)
	}
}

// A map target has no notion of an unknown key, so strict mode is a no-op there.
func TestUnknownFieldsMapTargetUnaffected(t *testing.T) {
	var m map[string]any
	if err := setay.Unmarshal([]byte(`{ a = 1; b = 2; }`), &m, setay.DisallowUnknownFields()); err != nil {
		t.Fatalf("strict mode should not error for a map target: %v", err)
	}
	if len(m) != 2 {
		t.Errorf("map = %v, want 2 entries", m)
	}
}

// A key that matches a field only case-insensitively is not "unknown".
func TestUnknownFieldsCaseInsensitiveMatch(t *testing.T) {
	type doc struct {
		Mode string
	}
	var d doc
	if err := setay.Unmarshal([]byte(`{ MODE = "x"; }`), &d, setay.DisallowUnknownFields()); err != nil {
		t.Fatalf("case-insensitive match should not be reported as unknown: %v", err)
	}
	if d.Mode != "x" {
		t.Errorf("Mode = %q, want x", d.Mode)
	}
}
