package setay_test

import (
	setay "github.com/aki-kuramoto/setay-go"
	"github.com/aki-kuramoto/wantai"
	"strings"
	"testing"
	"time"
)

type BasicConfig struct {
	Name      string  `setay:"name"`
	Age       int     `setay:"age"`
	Score     float64 `setay:"score"`
	IsActive  bool    `setay:"is-active"`
	Hidden    string  `setay:"-"`
	NoTag     string
	OptField  string  `setay:"opt,omitempty"`
}

type NestedConfig struct {
	Title   string       `setay:"title"`
	Server  ServerConfig `setay:"server"`
	Tags    []string     `setay:"tags"`
	Numbers []int        `setay:"numbers"`
}

type ServerConfig struct {
	Host string `setay:"host"`
	Port int    `setay:"port"`
}

type TimeConfig struct {
	Created  time.Time  `setay:"created"`
	Modified *time.Time `setay:"modified,omitempty"`
}

// Test basic Marshal
func TestMarshalBasic(t *testing.T) {
	cfg := BasicConfig{
		Name:     "John",
		Age:      42,
		Score:    3.14,
		IsActive: true,
		Hidden:   "should not appear",
		NoTag:    "visible",
		OptField: "",
	}
	data, err := setay.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	out := string(data)
	t.Logf("Marshal output:\n%s", out)

	// Verify key aspects
	if !strings.Contains(out, `name = "John"`) {
		t.Error("missing name field")
	}
	if !strings.Contains(out, `age = 42`) {
		t.Error("missing age field")
	}
	if !strings.Contains(out, `score = 3.14`) {
		t.Error("missing score field")
	}
	if !strings.Contains(out, `is-active = true`) {
		t.Error("missing is-active field")
	}
	if strings.Contains(out, "should not appear") {
		t.Error("hidden field should not appear")
	}
	if !strings.Contains(out, `NoTag = "visible"`) {
		t.Error("NoTag field should use field name as key")
	}
	if strings.Contains(out, "opt") {
		t.Error("omitempty field with zero value should not appear")
	}
}

// Test basic Unmarshal
func TestUnmarshalBasic(t *testing.T) {
	input := `{
	name = "Alice";
	age = 30;
	score = 9.5;
	is-active = false;
	NoTag = "hello"
}`
	var cfg BasicConfig
	err := setay.Unmarshal([]byte(input), &cfg)
	if err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if cfg.Name != "Alice" {
		t.Errorf("Name = %q, want %q", cfg.Name, "Alice")
	}
	if cfg.Age != 30 {
		t.Errorf("Age = %d, want 30", cfg.Age)
	}
	if cfg.Score != 9.5 {
		t.Errorf("Score = %f, want 9.5", cfg.Score)
	}
	if cfg.IsActive != false {
		t.Error("IsActive should be false")
	}
	if cfg.NoTag != "hello" {
		t.Errorf("NoTag = %q, want %q", cfg.NoTag, "hello")
	}
}

// Test nested struct
func TestMarshalNested(t *testing.T) {
	cfg := NestedConfig{
		Title: "My App",
		Server: ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
		Tags:    []string{"web", "api"},
		Numbers: []int{1, 2, 3},
	}
	data, err := setay.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	t.Logf("Nested output:\n%s", string(data))

	if !strings.Contains(string(data), `host = "localhost"`) {
		t.Error("missing nested host")
	}
}

// Test roundtrip (Marshal → Unmarshal)
func TestRoundtrip(t *testing.T) {
	original := BasicConfig{
		Name:     "Test User",
		Age:      25,
		Score:    100.0,
		IsActive: true,
		NoTag:    "roundtrip",
	}

	data, err := setay.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	t.Logf("Roundtrip data:\n%s", string(data))

	var decoded BasicConfig
	err = setay.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.Name != original.Name {
		t.Errorf("Name: got %q, want %q", decoded.Name, original.Name)
	}
	if decoded.Age != original.Age {
		t.Errorf("Age: got %d, want %d", decoded.Age, original.Age)
	}
	if decoded.Score != original.Score {
		t.Errorf("Score: got %f, want %f", decoded.Score, original.Score)
	}
	if decoded.IsActive != original.IsActive {
		t.Errorf("IsActive: got %v, want %v", decoded.IsActive, original.IsActive)
	}
}

// Test roundtrip with nested
func TestRoundtripNested(t *testing.T) {
	original := NestedConfig{
		Title: "Nested Test",
		Server: ServerConfig{
			Host: "example.com",
			Port: 443,
		},
		Tags:    []string{"https", "prod"},
		Numbers: []int{10, 20, 30},
	}

	data, err := setay.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	t.Logf("Nested roundtrip:\n%s", string(data))

	var decoded NestedConfig
	err = setay.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.Title != original.Title {
		t.Errorf("Title: got %q, want %q", decoded.Title, original.Title)
	}
	if decoded.Server.Host != original.Server.Host {
		t.Errorf("Host: got %q, want %q", decoded.Server.Host, original.Server.Host)
	}
	if decoded.Server.Port != original.Server.Port {
		t.Errorf("Port: got %d, want %d", decoded.Server.Port, original.Server.Port)
	}
	if len(decoded.Tags) != len(original.Tags) {
		t.Errorf("Tags length: got %d, want %d", len(decoded.Tags), len(original.Tags))
	}
}

// Test time.Time roundtrip
func TestRoundtripTime(t *testing.T) {
	now := time.Date(2026, 3, 22, 12, 30, 0, 0, time.UTC)
	original := TimeConfig{
		Created: now,
	}

	data, err := setay.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	t.Logf("Time output:\n%s", string(data))

	if !strings.Contains(string(data), `UtcTs("2026-03-22 12:30:00")`) {
		t.Error("time.Time should be serialized as UtcTs(...)")
	}

	var decoded TimeConfig
	err = setay.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if !decoded.Created.Equal(original.Created) {
		t.Errorf("Created: got %v, want %v", decoded.Created, original.Created)
	}
}

// Test map[string]interface{}
func TestUnmarshalToMap(t *testing.T) {
	input := `{
	name = "Bob";
	age = 99;
	score = 2.5;
	active = true;
	nothing = null;
	items = [ 1, 2, 3 ]
}`
	var m map[string]interface{}
	err := setay.Unmarshal([]byte(input), &m)
	if err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if m["name"] != "Bob" {
		t.Errorf("name = %v, want Bob", m["name"])
	}
	if m["age"] != int64(99) {
		t.Errorf("age = %v (%T), want 99", m["age"], m["age"])
	}
	if m["active"] != true {
		t.Errorf("active = %v, want true", m["active"])
	}
	if m["nothing"] != nil {
		t.Errorf("nothing = %v, want nil", m["nothing"])
	}

	items, ok := m["items"].([]interface{})
	if !ok || len(items) != 3 {
		t.Errorf("items = %v, want [1,2,3]", m["items"])
	}
}

// Test null pointer
func TestMarshalNilPointer(t *testing.T) {
	type WithPtr struct {
		Name  string  `setay:"name"`
		Value *int    `setay:"value"`
	}
	cfg := WithPtr{Name: "test", Value: nil}
	data, err := setay.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	t.Logf("Nil pointer output:\n%s", string(data))

	if !strings.Contains(string(data), "value = null") {
		t.Error("nil pointer should be serialized as null")
	}
}

// Test escape roundtrip
func TestEscapeRoundtrip(t *testing.T) {
	type EscTest struct {
		Text string `setay:"text"`
	}
	original := EscTest{Text: "hello\tworld\nnewline\"quote\\backslash"}
	data, err := setay.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	t.Logf("Escape output:\n%s", string(data))

	var decoded EscTest
	err = setay.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if decoded.Text != original.Text {
		t.Errorf("Text: got %q, want %q", decoded.Text, original.Text)
	}
}

// Test number prefixes
func TestUnmarshalNumberPrefixes(t *testing.T) {
	input := `{
	hex = 0xFF;
	bin = 0b1010;
	oct = 0o77;
	dec_pref = 0d42;
	neg = -100;
	leading = 008
}`
	type NumConfig struct {
		Hex     int `setay:"hex"`
		Bin     int `setay:"bin"`
		Oct     int `setay:"oct"`
		DecPref int `setay:"dec_pref"`
		Neg     int `setay:"neg"`
		Leading int `setay:"leading"`
	}
	var cfg NumConfig
	err := setay.Unmarshal([]byte(input), &cfg)
	if err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if cfg.Hex != 0xFF {
		t.Errorf("Hex = %d, want 255", cfg.Hex)
	}
	if cfg.Bin != 0b1010 {
		t.Errorf("Bin = %d, want 10", cfg.Bin)
	}
	if cfg.Oct != 0o77 {
		t.Errorf("Oct = %d, want 63", cfg.Oct)
	}
	if cfg.DecPref != 42 {
		t.Errorf("DecPref = %d, want 42", cfg.DecPref)
	}
	if cfg.Neg != -100 {
		t.Errorf("Neg = %d, want -100", cfg.Neg)
	}
	if cfg.Leading != 8 {
		t.Errorf("Leading = %d, want 8", cfg.Leading)
	}
}

// --- wantai timestamp type tests ---

type WantaiConfig struct {
	CreatedNano  wantai.UtcNanoTs  `setay:"created-nano"`
	CreatedMilli wantai.UtcMilliTs `setay:"created-milli"`
}

func TestMarshalWantaiTimestamp(t *testing.T) {
	refTime := time.Date(2026, 3, 22, 12, 28, 27, 0, time.UTC)
	cfg := WantaiConfig{
		CreatedNano:  wantai.FromTime(refTime),
		CreatedMilli: wantai.FromTimeMillis(refTime),
	}
	data, err := setay.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	out := string(data)
	t.Logf("Marshal output:\n%s", out)

	if !strings.Contains(out, `created-nano = UtcTs("2026-03-22 12:28:27")`) {
		t.Error("wantai UtcNanoTs should marshal as UtcTs")
	}
	if !strings.Contains(out, `created-milli = UtcTs("2026-03-22 12:28:27")`) {
		t.Error("wantai UtcMilliTs should marshal as UtcTs")
	}
}

func TestUnmarshalWantaiTimestamp(t *testing.T) {
	input := []byte(`{
		created-nano = UtcTs("2026-03-22 12:28:27");
		created-milli = UtcTs("2026-03-22 12:28:27")
	}`)
	var cfg WantaiConfig
	err := setay.Unmarshal(input, &cfg)
	if err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	refTime := time.Date(2026, 3, 22, 12, 28, 27, 0, time.UTC)

	// UtcNanoTs: should match UnixNano of the reference time
	expectedNano := wantai.FromTime(refTime)
	if cfg.CreatedNano != expectedNano {
		t.Errorf("CreatedNano = %d, want %d", cfg.CreatedNano, expectedNano)
	}

	// UtcMilliTs: should match UnixMilli of the reference time
	expectedMilli := wantai.FromTimeMillis(refTime)
	if cfg.CreatedMilli != expectedMilli {
		t.Errorf("CreatedMilli = %d, want %d", cfg.CreatedMilli, expectedMilli)
	}

	// Verify ToTime() round-trip
	if !cfg.CreatedNano.ToTime().Equal(refTime) {
		t.Errorf("CreatedNano.ToTime() = %v, want %v", cfg.CreatedNano.ToTime(), refTime)
	}
	if !cfg.CreatedMilli.ToTime().Equal(refTime) {
		t.Errorf("CreatedMilli.ToTime() = %v, want %v", cfg.CreatedMilli.ToTime(), refTime)
	}
}

func TestRoundTripWantaiTimestamp(t *testing.T) {
	refTime := time.Date(2026, 3, 22, 12, 28, 27, 0, time.UTC)
	original := WantaiConfig{
		CreatedNano:  wantai.FromTime(refTime),
		CreatedMilli: wantai.FromTimeMillis(refTime),
	}

	data, err := setay.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var loaded WantaiConfig
	err = setay.Unmarshal(data, &loaded)
	if err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if !loaded.CreatedNano.ToTime().Equal(original.CreatedNano.ToTime()) {
		t.Errorf("Round-trip UtcNanoTs mismatch: %v != %v",
			loaded.CreatedNano.ToTime(), original.CreatedNano.ToTime())
	}
	if !loaded.CreatedMilli.ToTime().Equal(original.CreatedMilli.ToTime()) {
		t.Errorf("Round-trip UtcMilliTs mismatch: %v != %v",
			loaded.CreatedMilli.ToTime(), original.CreatedMilli.ToTime())
	}
}
