package setay_test

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	setay "github.com/aki-kuramoto/setay-go"
	"github.com/aki-kuramoto/wantai"
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

// =====================================================================
//  Variable Reference (${VAR}) Tests
// =====================================================================

type VarConfig struct {
	Host     string `setay:"host"`
	Port     int    `setay:"port"`
	Password string `setay:"password"`
	Key      string `setay:"key"`
	Greeting string `setay:"greeting"`
	Debug    bool   `setay:"debug"`
}

// TestVarEnvDefault: ${VAR} resolves from environment variable.
func TestVarEnvDefault(t *testing.T) {
	t.Setenv("SETAY_TEST_HOST", "env-host.example.com")

	input := `{ host = ${SETAY_TEST_HOST} }`
	var cfg VarConfig
	if err := setay.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg.Host != "env-host.example.com" {
		t.Errorf("Host = %q, want %q", cfg.Host, "env-host.example.com")
	}
}

// TestVarCustomResolverOk: custom resolver ok=true takes priority over env.
func TestVarCustomResolverOk(t *testing.T) {
	t.Setenv("SETAY_TEST_HOST", "env-value")
	setay.RegisterVariableResolver(func(name string) (string, bool, error) {
		if name == "SETAY_TEST_HOST" {
			return "custom-value", true, nil
		}
		return "", false, nil
	})
	defer setay.RegisterVariableResolver(nil)

	input := `{ host = ${SETAY_TEST_HOST} }`
	var cfg VarConfig
	if err := setay.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg.Host != "custom-value" {
		t.Errorf("Host = %q, want %q", cfg.Host, "custom-value")
	}
}

// TestVarCustomResolverFallsToEnv: resolver ok=false falls through to env.
func TestVarCustomResolverFallsToEnv(t *testing.T) {
	t.Setenv("SETAY_TEST_HOST", "fallback-env")
	setay.RegisterVariableResolver(func(name string) (string, bool, error) {
		// Always return ok=false → env should be used.
		return "", false, nil
	})
	defer setay.RegisterVariableResolver(nil)

	input := `{ host = ${SETAY_TEST_HOST} }`
	var cfg VarConfig
	if err := setay.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg.Host != "fallback-env" {
		t.Errorf("Host = %q, want %q", cfg.Host, "fallback-env")
	}
}

// TestVarNumberFallback: ${MISSING ?: 9999} uses integer literal fallback.
func TestVarNumberFallback(t *testing.T) {
	os.Unsetenv("SETAY_MISSING_PORT")

	input := `{ port = ${SETAY_MISSING_PORT ?: 9999} }`
	var cfg VarConfig
	if err := setay.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg.Port != 9999 {
		t.Errorf("Port = %d, want 9999", cfg.Port)
	}
}

// TestVarStringFallback: ${MISSING ?: "fallback"} uses string literal fallback.
func TestVarStringFallback(t *testing.T) {
	os.Unsetenv("SETAY_MISSING_KEY")

	input := `{ key = ${SETAY_MISSING_KEY ?: "my-default-key"} }`
	var cfg VarConfig
	if err := setay.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg.Key != "my-default-key" {
		t.Errorf("Key = %q, want %q", cfg.Key, "my-default-key")
	}
}

// TestVarFallbackChain: ${A ?: B ?: "last"} tries in order.
func TestVarFallbackChain(t *testing.T) {
	os.Unsetenv("SETAY_CHAIN_A")
	os.Unsetenv("SETAY_CHAIN_B")

	input := `{ host = ${SETAY_CHAIN_A ?: SETAY_CHAIN_B ?: "last-resort"} }`
	var cfg VarConfig
	if err := setay.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg.Host != "last-resort" {
		t.Errorf("Host = %q, want %q", cfg.Host, "last-resort")
	}
}

// TestVarFallbackChainMiddle: second variable in chain is set.
func TestVarFallbackChainMiddle(t *testing.T) {
	os.Unsetenv("SETAY_CHAIN_A")
	t.Setenv("SETAY_CHAIN_B", "from-B")

	input := `{ host = ${SETAY_CHAIN_A ?: SETAY_CHAIN_B ?: "last-resort"} }`
	var cfg VarConfig
	if err := setay.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg.Host != "from-B" {
		t.Errorf("Host = %q, want %q", cfg.Host, "from-B")
	}
}

// TestVarStringInterpolationMiddle: "prefix-${VAR}-suffix" expansion.
func TestVarStringInterpolationMiddle(t *testing.T) {
	t.Setenv("SETAY_INTERP_VAR", "middle")

	input := `{ greeting = "prefix-${SETAY_INTERP_VAR}-suffix" }`
	var cfg VarConfig
	if err := setay.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	want := "prefix-middle-suffix"
	if cfg.Greeting != want {
		t.Errorf("Greeting = %q, want %q", cfg.Greeting, want)
	}
}

// TestVarStringInterpolationMultiple: multiple variables expanded in one string.
func TestVarStringInterpolationMultiple(t *testing.T) {
	t.Setenv("SETAY_INTERP_A", "Hello")
	t.Setenv("SETAY_INTERP_B", "world")

	// Both variables are set — both expand in the same string.
	input := `{ greeting = "${SETAY_INTERP_A}, ${SETAY_INTERP_B}!" }`
	var cfg VarConfig
	if err := setay.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	want := "Hello, world!"
	if cfg.Greeting != want {
		t.Errorf("Greeting = %q, want %q", cfg.Greeting, want)
	}
}

// TestVarStringInterpolationWithFallback: fallback with single-quoted string inside double-quoted interpolation.
func TestVarStringInterpolationWithFallback(t *testing.T) {
	t.Setenv("SETAY_INTERP_A", "Hello")
	os.Unsetenv("SETAY_INTERP_MISSING")

	// ${SETAY_INTERP_MISSING ?: 'default'} uses single-quoted string as fallback inside double-quoted string.
	input := `{ greeting = "${SETAY_INTERP_A}, ${SETAY_INTERP_MISSING ?: 'world'}!" }`
	var cfg VarConfig
	if err := setay.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	want := "Hello, world!"
	if cfg.Greeting != want {
		t.Errorf("Greeting = %q, want %q", cfg.Greeting, want)
	}
}

// TestVarInterpolationFallbackNumber: number fallback inside string interpolation.
func TestVarInterpolationFallbackNumber(t *testing.T) {
	os.Unsetenv("SETAY_INTERP_PORT")

	input := `{ greeting = "Port is ${SETAY_INTERP_PORT ?: 8080}" }`
	var cfg VarConfig
	if err := setay.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	want := "Port is 8080"
	if cfg.Greeting != want {
		t.Errorf("Greeting = %q, want %q", cfg.Greeting, want)
	}
}

// TestVarUndefinedError: undefined variable with no fallback causes an error.
func TestVarUndefinedError(t *testing.T) {
	os.Unsetenv("SETAY_UNDEFINED_XYZZY")

	input := `{ host = ${SETAY_UNDEFINED_XYZZY} }`
	var cfg VarConfig
	err := setay.Unmarshal([]byte(input), &cfg)
	if err == nil {
		t.Fatal("Expected error for undefined variable, got nil")
	}
	if !strings.Contains(err.Error(), "SETAY_UNDEFINED_XYZZY") {
		t.Errorf("Error should mention the variable name, got: %v", err)
	}
}

// TestVarResolverError: resolver returning an error propagates to Unmarshal.
func TestVarResolverError(t *testing.T) {
	setay.RegisterVariableResolver(func(name string) (string, bool, error) {
		return "", false, fmt.Errorf("injected resolver error")
	})
	defer setay.RegisterVariableResolver(nil)

	input := `{ host = ${SOME_VAR} }`
	var cfg VarConfig
	err := setay.Unmarshal([]byte(input), &cfg)
	if err == nil {
		t.Fatal("Expected error from resolver, got nil")
	}
	if !strings.Contains(err.Error(), "injected resolver error") {
		t.Errorf("Error should contain resolver error message, got: %v", err)
	}
}

// TestVarResolverReset: RegisterVariableResolver(nil) resets to env-only mode.
func TestVarResolverReset(t *testing.T) {
	calledWith := ""
	setay.RegisterVariableResolver(func(name string) (string, bool, error) {
		calledWith = name
		return "custom", true, nil
	})

	// Verify resolver is active.
	t.Setenv("SETAY_RESET_TEST", "env-value")
	input := `{ host = ${SETAY_RESET_TEST} }`
	var cfg1 VarConfig
	if err := setay.Unmarshal([]byte(input), &cfg1); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg1.Host != "custom" {
		t.Errorf("Before reset: Host = %q, want %q", cfg1.Host, "custom")
	}
	if calledWith != "SETAY_RESET_TEST" {
		t.Errorf("Resolver was not called, calledWith = %q", calledWith)
	}

	// Reset resolver.
	setay.RegisterVariableResolver(nil)

	// Now env variable should be used.
	var cfg2 VarConfig
	if err := setay.Unmarshal([]byte(input), &cfg2); err != nil {
		t.Fatalf("Unmarshal after reset error: %v", err)
	}
	if cfg2.Host != "env-value" {
		t.Errorf("After reset: Host = %q, want %q", cfg2.Host, "env-value")
	}
}

// TestVarBoolFallback: variable with bool fallback.
func TestVarBoolFallback(t *testing.T) {
	os.Unsetenv("SETAY_DEBUG_FLAG")

	// Note: bool fallback uses "true"/"false" string which setFromString handles.
	input := `{ debug = ${SETAY_DEBUG_FLAG ?: "true"} }`
	var cfg VarConfig
	if err := setay.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if !cfg.Debug {
		t.Errorf("Debug = false, want true")
	}
}

// TestVarEscapedDollarLiteral: in a double-quoted string a literal '$' must be
// escaped (\$); a bare '$' is reserved (only '${...}' is interpolation), so a
// bare '$' that is not '${' is now a parse error.
func TestVarEscapedDollarLiteral(t *testing.T) {
	input := `{ greeting = "price: \$100" }`
	var cfg VarConfig
	if err := setay.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg.Greeting != "price: $100" {
		t.Errorf("Greeting = %q, want %q", cfg.Greeting, "price: $100")
	}
	if err := setay.Unmarshal([]byte(`{ g = "a $ b" }`), &cfg); err == nil {
		t.Error("bare '$' in a double-quoted string should now be a parse error")
	}
}

// ======================================================================
//  Set tests
// ======================================================================

// TestUnmarshalSetInt: basic integer set.
func TestUnmarshalSetInt(t *testing.T) {
	input := `{
		ids =
			{
				1=;
				7=;
				13=;
				111=;
			}
	}`
	var cfg struct {
		IDs map[int]struct{} `setay:"ids"`
	}
	if err := setay.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	want := map[int]struct{}{1: {}, 7: {}, 13: {}, 111: {}}
	if len(cfg.IDs) != len(want) {
		t.Fatalf("IDs len = %d, want %d", len(cfg.IDs), len(want))
	}
	for k := range want {
		if _, ok := cfg.IDs[k]; !ok {
			t.Errorf("IDs missing key %d", k)
		}
	}
}

// TestUnmarshalSetString: string set.
func TestUnmarshalSetString(t *testing.T) {
	input := `{ tags = { "go"=; "setay"=; "open-source"=; } }`
	var cfg struct {
		Tags map[string]struct{} `setay:"tags"`
	}
	if err := setay.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	want := map[string]struct{}{"go": {}, "setay": {}, "open-source": {}}
	if len(cfg.Tags) != len(want) {
		t.Fatalf("Tags len = %d, want %d", len(cfg.Tags), len(want))
	}
	for k := range want {
		if _, ok := cfg.Tags[k]; !ok {
			t.Errorf("Tags missing key %q", k)
		}
	}
}

// TestUnmarshalSetEmpty: {=;} is an empty set.
func TestUnmarshalSetEmpty(t *testing.T) {
	input := `{ flags = {=;} }`
	var cfg struct {
		Flags map[string]struct{} `setay:"flags"`
	}
	if err := setay.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if len(cfg.Flags) != 0 {
		t.Errorf("Flags len = %d, want 0", len(cfg.Flags))
	}
}

// TestUnmarshalSetEmptyWithComments: {=;} with comments is still an empty set.
func TestUnmarshalSetEmptyWithComments(t *testing.T) {
	input := "{ flags = { #{ a comment }# =; #{ another }# } }"
	var cfg struct {
		Flags map[int]struct{} `setay:"flags"`
	}
	if err := setay.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if len(cfg.Flags) != 0 {
		t.Errorf("Flags len = %d, want 0", len(cfg.Flags))
	}
}

// TestUnmarshalSetSpaceBeforeSuffix: space before =; is allowed.
func TestUnmarshalSetSpaceBeforeSuffix(t *testing.T) {
	input := `{ ids = { 1 =; 2 =; 3 =; } }`
	var cfg struct {
		IDs map[int]struct{} `setay:"ids"`
	}
	if err := setay.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if len(cfg.IDs) != 3 {
		t.Fatalf("IDs len = %d, want 3", len(cfg.IDs))
	}
}

// TestUnmarshalSetFloat: float set.
func TestUnmarshalSetFloat(t *testing.T) {
	input := `{ scores = { 1.5=; 2.5=; 3.14=; } }`
	var cfg struct {
		Scores map[float64]struct{} `setay:"scores"`
	}
	if err := setay.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	want := map[float64]struct{}{1.5: {}, 2.5: {}, 3.14: {}}
	if len(cfg.Scores) != len(want) {
		t.Fatalf("Scores len = %d, want %d", len(cfg.Scores), len(want))
	}
	for k := range want {
		if _, ok := cfg.Scores[k]; !ok {
			t.Errorf("Scores missing key %v", k)
		}
	}
}

// TestUnmarshalSetBool: bool set.
func TestUnmarshalSetBool(t *testing.T) {
	input := `{ flags = { true=; false=; } }`
	var cfg struct {
		Flags map[bool]struct{} `setay:"flags"`
	}
	if err := setay.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if len(cfg.Flags) != 2 {
		t.Fatalf("Flags len = %d, want 2", len(cfg.Flags))
	}
}

// TestUnmarshalSetHexKey: hex integer key in a set.
func TestUnmarshalSetHexKey(t *testing.T) {
	input := `{ ids = { 0xFF=; 0x10=; } }`
	var cfg struct {
		IDs map[int]struct{} `setay:"ids"`
	}
	if err := setay.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	want := map[int]struct{}{255: {}, 16: {}}
	if len(cfg.IDs) != len(want) {
		t.Fatalf("IDs len = %d, want %d", len(cfg.IDs), len(want))
	}
	for k := range want {
		if _, ok := cfg.IDs[k]; !ok {
			t.Errorf("IDs missing key %d", k)
		}
	}
}

// TestUnmarshalSetErrorWrongValueType: map[K]string cannot receive a set.
func TestUnmarshalSetErrorWrongValueType(t *testing.T) {
	input := `{ ids = { 1=; 2=; } }`
	var cfg struct {
		IDs map[int]string `setay:"ids"`
	}
	err := setay.Unmarshal([]byte(input), &cfg)
	if err == nil {
		t.Fatal("Expected error for map[int]string receiving a set, got nil")
	}
}

// TestMarshalSetInt: marshal map[int]struct{} as set.
func TestMarshalSetInt(t *testing.T) {
	type Cfg struct {
		IDs map[int]struct{} `setay:"ids"`
	}
	cfg := Cfg{IDs: map[int]struct{}{1: {}, 7: {}}}
	data, err := setay.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, "1=;") || !strings.Contains(s, "7=;") {
		t.Errorf("Marshal output missing set entries:\n%s", s)
	}
	// Must NOT contain " = 1" style (dict format)
	if strings.Contains(s, "ids = 1") {
		t.Errorf("Marshal output looks like dict, not set:\n%s", s)
	}
}

// TestMarshalSetString: marshal map[string]struct{} as set.
func TestMarshalSetString(t *testing.T) {
	type Cfg struct {
		Tags map[string]struct{} `setay:"tags"`
	}
	cfg := Cfg{Tags: map[string]struct{}{"go": {}, "setay": {}}}
	data, err := setay.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, `"go"=;`) && !strings.Contains(s, `"setay"=;`) {
		t.Errorf("Marshal output missing set entries:\n%s", s)
	}
}

// TestMarshalSetEmpty: empty map[K]struct{} marshals as {=;}.
func TestMarshalSetEmpty(t *testing.T) {
	type Cfg struct {
		Tags map[string]struct{} `setay:"tags"`
	}
	cfg := Cfg{Tags: map[string]struct{}{}}
	data, err := setay.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, "{=;}") {
		t.Errorf("Marshal output missing {=;} for empty set:\n%s", s)
	}
}

// TestMarshalSetTopLevelError: top-level set should return an error.
func TestMarshalSetTopLevelError(t *testing.T) {
	_, err := setay.Marshal(map[int]struct{}{1: {}, 2: {}})
	if err == nil {
		t.Fatal("Expected error for top-level set marshal, got nil")
	}
}

// TestRoundTripSet: marshal then unmarshal a set preserves all elements.
func TestRoundTripSet(t *testing.T) {
	type Cfg struct {
		Ports map[int]struct{} `setay:"ports"`
	}
	original := Cfg{Ports: map[int]struct{}{80: {}, 443: {}, 8080: {}}}

	data, err := setay.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var restored Cfg
	if err := setay.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal error: %v\nInput:\n%s", err, data)
	}

	if len(restored.Ports) != len(original.Ports) {
		t.Fatalf("Ports len = %d, want %d\nsetay:\n%s", len(restored.Ports), len(original.Ports), data)
	}
	for k := range original.Ports {
		if _, ok := restored.Ports[k]; !ok {
			t.Errorf("Ports missing key %d\nsetay:\n%s", k, data)
		}
	}
}

