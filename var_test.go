package setay_test

// Comprehensive unit tests for the ${VAR} variable reference feature.
// These tests complement the basic smoke tests in setay_test.go and focus on:
//   - Type coercion (env string value → various Go types)
//   - Fallback literal types (negative int, float, hex, binary, octal)
//   - Collections: variables inside lists and map values
//   - Single-quoted strings: no interpolation
//   - Nested struct/map traversal
//   - Edge cases: empty env var, whitespace inside ${}, pointer targets
//   - Resolver call-count and argument verification

import (
	"fmt"
	"os"
	"strings"
	"testing"

	setay "github.com/aki-kuramoto/setay-go"
)

// =====================================================================
//  Shared helper structs for type-coercion tests
// =====================================================================

type VarTypedConfig struct {
	StrVal   string      `setay:"str-val"`
	IntVal   int         `setay:"int-val"`
	Int8Val  int8        `setay:"int8-val"`
	Int16Val int16       `setay:"int16-val"`
	Int32Val int32       `setay:"int32-val"`
	Int64Val int64       `setay:"int64-val"`
	UintVal  uint        `setay:"uint-val"`
	Uint32   uint32      `setay:"uint32-val"`
	FloatVal float64     `setay:"float-val"`
	Float32  float32     `setay:"float32-val"`
	BoolVal  bool        `setay:"bool-val"`
	AnyVal   interface{} `setay:"any-val"`
	PtrStr   *string     `setay:"ptr-str"`
	PtrInt   *int        `setay:"ptr-int"`
}

type VarNestedOuter struct {
	Title  string         `setay:"title"`
	Server VarNestedInner `setay:"server"`
}

type VarNestedInner struct {
	Host string `setay:"host"`
	Port int    `setay:"port"`
}

// =====================================================================
//  Category 1: Type coercion — env var (string) → various Go types
// =====================================================================

// TestVarCoerceString: env var → string field.
func TestVarCoerceString(t *testing.T) {
	t.Setenv("SETAY_TC_STR", "hello-world")
	var cfg VarTypedConfig
	if err := setay.Unmarshal([]byte(`{ str-val = ${SETAY_TC_STR} }`), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg.StrVal != "hello-world" {
		t.Errorf("StrVal = %q, want %q", cfg.StrVal, "hello-world")
	}
}

// TestVarCoerceInt: env var "42" → int field.
func TestVarCoerceInt(t *testing.T) {
	t.Setenv("SETAY_TC_INT", "42")
	var cfg VarTypedConfig
	if err := setay.Unmarshal([]byte(`{ int-val = ${SETAY_TC_INT} }`), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg.IntVal != 42 {
		t.Errorf("IntVal = %d, want 42", cfg.IntVal)
	}
}

// TestVarCoerceInt64: env var "-1000000" → int64 field.
func TestVarCoerceInt64(t *testing.T) {
	t.Setenv("SETAY_TC_INT64", "-1000000")
	var cfg VarTypedConfig
	if err := setay.Unmarshal([]byte(`{ int64-val = ${SETAY_TC_INT64} }`), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg.Int64Val != -1000000 {
		t.Errorf("Int64Val = %d, want -1000000", cfg.Int64Val)
	}
}

// TestVarCoerceUint: env var "255" → uint field.
func TestVarCoerceUint(t *testing.T) {
	t.Setenv("SETAY_TC_UINT", "255")
	var cfg VarTypedConfig
	if err := setay.Unmarshal([]byte(`{ uint-val = ${SETAY_TC_UINT} }`), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg.UintVal != 255 {
		t.Errorf("UintVal = %d, want 255", cfg.UintVal)
	}
}

// TestVarCoerceFloat64: env var "3.14" → float64 field.
func TestVarCoerceFloat64(t *testing.T) {
	t.Setenv("SETAY_TC_FLOAT", "3.14")
	var cfg VarTypedConfig
	if err := setay.Unmarshal([]byte(`{ float-val = ${SETAY_TC_FLOAT} }`), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg.FloatVal != 3.14 {
		t.Errorf("FloatVal = %f, want 3.14", cfg.FloatVal)
	}
}

// TestVarCoerceBoolTrue: env var "true" → bool field.
func TestVarCoerceBoolTrue(t *testing.T) {
	t.Setenv("SETAY_TC_BOOL", "true")
	var cfg VarTypedConfig
	if err := setay.Unmarshal([]byte(`{ bool-val = ${SETAY_TC_BOOL} }`), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if !cfg.BoolVal {
		t.Error("BoolVal = false, want true")
	}
}

// TestVarCoerceBoolFalse: env var "false" → bool field.
func TestVarCoerceBoolFalse(t *testing.T) {
	t.Setenv("SETAY_TC_BOOL2", "false")
	var cfg VarTypedConfig
	if err := setay.Unmarshal([]byte(`{ bool-val = ${SETAY_TC_BOOL2} }`), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg.BoolVal {
		t.Error("BoolVal = true, want false")
	}
}

// TestVarCoerceBoolNumeric1: env var "1" → bool=true.
func TestVarCoerceBoolNumeric1(t *testing.T) {
	t.Setenv("SETAY_TC_BOOL3", "1")
	var cfg VarTypedConfig
	if err := setay.Unmarshal([]byte(`{ bool-val = ${SETAY_TC_BOOL3} }`), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if !cfg.BoolVal {
		t.Error("BoolVal = false, want true (from '1')")
	}
}

// TestVarCoerceBoolNumeric0: env var "0" → bool=false.
func TestVarCoerceBoolNumeric0(t *testing.T) {
	t.Setenv("SETAY_TC_BOOL4", "0")
	var cfg VarTypedConfig
	if err := setay.Unmarshal([]byte(`{ bool-val = ${SETAY_TC_BOOL4} }`), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg.BoolVal {
		t.Error("BoolVal = true, want false (from '0')")
	}
}

// TestVarCoerceBoolYes: env var "yes" → bool=true.
func TestVarCoerceBoolYes(t *testing.T) {
	t.Setenv("SETAY_TC_BOOL5", "yes")
	var cfg VarTypedConfig
	if err := setay.Unmarshal([]byte(`{ bool-val = ${SETAY_TC_BOOL5} }`), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if !cfg.BoolVal {
		t.Error("BoolVal = false, want true (from 'yes')")
	}
}

// TestVarCoerceBoolNo: env var "no" → bool=false.
func TestVarCoerceBoolNo(t *testing.T) {
	t.Setenv("SETAY_TC_BOOL6", "no")
	var cfg VarTypedConfig
	if err := setay.Unmarshal([]byte(`{ bool-val = ${SETAY_TC_BOOL6} }`), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg.BoolVal {
		t.Error("BoolVal = true, want false (from 'no')")
	}
}

// TestVarCoerceInterface: env var → interface{} gets a string value.
func TestVarCoerceInterface(t *testing.T) {
	t.Setenv("SETAY_TC_ANY", "dynamic-value")
	var cfg VarTypedConfig
	if err := setay.Unmarshal([]byte(`{ any-val = ${SETAY_TC_ANY} }`), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg.AnyVal != "dynamic-value" {
		t.Errorf("AnyVal = %v (%T), want string %q", cfg.AnyVal, cfg.AnyVal, "dynamic-value")
	}
}

// TestVarCoercePtrString: env var → *string pointer field.
func TestVarCoercePtrString(t *testing.T) {
	t.Setenv("SETAY_TC_PSTR", "ptr-content")
	var cfg VarTypedConfig
	if err := setay.Unmarshal([]byte(`{ ptr-str = ${SETAY_TC_PSTR} }`), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg.PtrStr == nil {
		t.Fatal("PtrStr is nil, want non-nil")
	}
	if *cfg.PtrStr != "ptr-content" {
		t.Errorf("*PtrStr = %q, want %q", *cfg.PtrStr, "ptr-content")
	}
}

// TestVarCoercePtrInt: env var "99" → *int pointer field.
func TestVarCoercePtrInt(t *testing.T) {
	t.Setenv("SETAY_TC_PINT", "99")
	var cfg VarTypedConfig
	if err := setay.Unmarshal([]byte(`{ ptr-int = ${SETAY_TC_PINT} }`), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg.PtrInt == nil {
		t.Fatal("PtrInt is nil, want non-nil")
	}
	if *cfg.PtrInt != 99 {
		t.Errorf("*PtrInt = %d, want 99", *cfg.PtrInt)
	}
}

// TestVarCoerceHexEnvToInt: env var "0xFF" → int 255.
func TestVarCoerceHexEnvToInt(t *testing.T) {
	t.Setenv("SETAY_TC_HEX", "0xFF")
	var cfg VarTypedConfig
	if err := setay.Unmarshal([]byte(`{ int-val = ${SETAY_TC_HEX} }`), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg.IntVal != 255 {
		t.Errorf("IntVal = %d, want 255", cfg.IntVal)
	}
}

// TestVarCoerceBadBool: env var with unrecognized bool string returns error.
func TestVarCoerceBadBool(t *testing.T) {
	t.Setenv("SETAY_TC_BADBOOL", "maybe")
	var cfg VarTypedConfig
	err := setay.Unmarshal([]byte(`{ bool-val = ${SETAY_TC_BADBOOL} }`), &cfg)
	if err == nil {
		t.Fatal("Expected error for unrecognized bool string, got nil")
	}
	if !strings.Contains(err.Error(), "bool") {
		t.Errorf("Error should mention bool, got: %v", err)
	}
}

// =====================================================================
//  Category 2: Fallback literal types
// =====================================================================

// TestVarFallbackNegativeInt: ${MISSING ?: -100} uses negative integer fallback.
func TestVarFallbackNegativeInt(t *testing.T) {
	os.Unsetenv("SETAY_FB_NEG")
	var cfg VarTypedConfig
	if err := setay.Unmarshal([]byte(`{ int-val = ${SETAY_FB_NEG ?: -100} }`), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg.IntVal != -100 {
		t.Errorf("IntVal = %d, want -100", cfg.IntVal)
	}
}

// TestVarFallbackFloatLiteral: ${MISSING ?: 3.14} uses float literal fallback.
func TestVarFallbackFloatLiteral(t *testing.T) {
	os.Unsetenv("SETAY_FB_FLOAT")
	var cfg VarTypedConfig
	if err := setay.Unmarshal([]byte(`{ float-val = ${SETAY_FB_FLOAT ?: 3.14} }`), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg.FloatVal != 3.14 {
		t.Errorf("FloatVal = %f, want 3.14", cfg.FloatVal)
	}
}

// TestVarFallbackHexLiteral: ${MISSING ?: 0xFF} uses hex literal fallback.
func TestVarFallbackHexLiteral(t *testing.T) {
	os.Unsetenv("SETAY_FB_HEX")
	var cfg VarTypedConfig
	if err := setay.Unmarshal([]byte(`{ int-val = ${SETAY_FB_HEX ?: 0xFF} }`), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg.IntVal != 255 {
		t.Errorf("IntVal = %d, want 255 (0xFF)", cfg.IntVal)
	}
}

// TestVarFallbackBinaryLiteral: ${MISSING ?: 0b1010} uses binary literal fallback.
func TestVarFallbackBinaryLiteral(t *testing.T) {
	os.Unsetenv("SETAY_FB_BIN")
	var cfg VarTypedConfig
	if err := setay.Unmarshal([]byte(`{ int-val = ${SETAY_FB_BIN ?: 0b1010} }`), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg.IntVal != 10 {
		t.Errorf("IntVal = %d, want 10 (0b1010)", cfg.IntVal)
	}
}

// TestVarFallbackSingleQuoteString: ${MISSING ?: 'fallback-sq'} uses single-quoted fallback.
func TestVarFallbackSingleQuoteString(t *testing.T) {
	os.Unsetenv("SETAY_FB_SQ")
	var cfg VarTypedConfig
	if err := setay.Unmarshal([]byte(`{ str-val = ${SETAY_FB_SQ ?: 'single-quoted'} }`), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg.StrVal != "single-quoted" {
		t.Errorf("StrVal = %q, want %q", cfg.StrVal, "single-quoted")
	}
}

// TestVarFallbackZero: ${MISSING ?: 0} uses zero as fallback.
func TestVarFallbackZero(t *testing.T) {
	os.Unsetenv("SETAY_FB_ZERO")
	var cfg VarTypedConfig
	if err := setay.Unmarshal([]byte(`{ int-val = ${SETAY_FB_ZERO ?: 0} }`), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg.IntVal != 0 {
		t.Errorf("IntVal = %d, want 0", cfg.IntVal)
	}
}

// TestVarFallbackLongChain: ${A ?: B ?: C ?: D ?: "found"} — all vars missing.
func TestVarFallbackLongChain(t *testing.T) {
	os.Unsetenv("SETAY_LC_A")
	os.Unsetenv("SETAY_LC_B")
	os.Unsetenv("SETAY_LC_C")
	os.Unsetenv("SETAY_LC_D")
	input := `{ str-val = ${SETAY_LC_A ?: SETAY_LC_B ?: SETAY_LC_C ?: SETAY_LC_D ?: "finally"} }`
	var cfg VarTypedConfig
	if err := setay.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg.StrVal != "finally" {
		t.Errorf("StrVal = %q, want %q", cfg.StrVal, "finally")
	}
}

// TestVarFallbackLongChainFirstHit: long chain where first var IS set.
func TestVarFallbackLongChainFirstHit(t *testing.T) {
	t.Setenv("SETAY_LCH_A", "from-A")
	os.Unsetenv("SETAY_LCH_B")
	os.Unsetenv("SETAY_LCH_C")
	input := `{ str-val = ${SETAY_LCH_A ?: SETAY_LCH_B ?: SETAY_LCH_C ?: "never"} }`
	var cfg VarTypedConfig
	if err := setay.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg.StrVal != "from-A" {
		t.Errorf("StrVal = %q, want %q", cfg.StrVal, "from-A")
	}
}

// =====================================================================
//  Category 3: Variables in collection types
// =====================================================================

// TestVarInList: ${VAR} elements inside a list.
func TestVarInList(t *testing.T) {
	t.Setenv("SETAY_LIST_A", "alpha")
	t.Setenv("SETAY_LIST_B", "beta")
	type SList struct {
		Items []string `setay:"items"`
	}
	input := `{ items = [ ${SETAY_LIST_A}, ${SETAY_LIST_B}, "literal" ] }`
	var cfg SList
	if err := setay.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	want := []string{"alpha", "beta", "literal"}
	if len(cfg.Items) != len(want) {
		t.Fatalf("Items length = %d, want %d", len(cfg.Items), len(want))
	}
	for i, w := range want {
		if cfg.Items[i] != w {
			t.Errorf("Items[%d] = %q, want %q", i, cfg.Items[i], w)
		}
	}
}

// TestVarInIntList: ${VAR} elements inside an int list.
func TestVarInIntList(t *testing.T) {
	t.Setenv("SETAY_ILIST_A", "10")
	t.Setenv("SETAY_ILIST_B", "20")
	type IList struct {
		Numbers []int `setay:"numbers"`
	}
	input := `{ numbers = [ ${SETAY_ILIST_A}, ${SETAY_ILIST_B}, 30 ] }`
	var cfg IList
	if err := setay.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	want := []int{10, 20, 30}
	if len(cfg.Numbers) != len(want) {
		t.Fatalf("Numbers length = %d, want %d", len(cfg.Numbers), len(want))
	}
	for i, w := range want {
		if cfg.Numbers[i] != w {
			t.Errorf("Numbers[%d] = %d, want %d", i, cfg.Numbers[i], w)
		}
	}
}

// TestVarInMapValue: map[string]string where values can be variables.
func TestVarInMapValue(t *testing.T) {
	t.Setenv("SETAY_MAP_HOST", "db.internal")
	os.Unsetenv("SETAY_MAP_MISSING")
	input := `{
		host = ${SETAY_MAP_HOST};
		mode = ${SETAY_MAP_MISSING ?: "default-mode"}
	}`
	var m map[string]string
	if err := setay.Unmarshal([]byte(input), &m); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if m["host"] != "db.internal" {
		t.Errorf("host = %q, want %q", m["host"], "db.internal")
	}
	if m["mode"] != "default-mode" {
		t.Errorf("mode = %q, want %q", m["mode"], "default-mode")
	}
}

// TestVarInInterfaceMap: map[string]interface{} with variable values.
func TestVarInInterfaceMap(t *testing.T) {
	t.Setenv("SETAY_INTF_NAME", "Alice")
	input := `{
		name = ${SETAY_INTF_NAME};
		title = "Engineer"
	}`
	var m map[string]interface{}
	if err := setay.Unmarshal([]byte(input), &m); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if m["name"] != "Alice" {
		t.Errorf("name = %v, want %q", m["name"], "Alice")
	}
	if m["title"] != "Engineer" {
		t.Errorf("title = %v, want %q", m["title"], "Engineer")
	}
}

// =====================================================================
//  Category 4: Nested structures
// =====================================================================

// TestVarInNestedStruct: variables in nested struct fields.
func TestVarInNestedStruct(t *testing.T) {
	t.Setenv("SETAY_NS_HOST", "nested-host")
	t.Setenv("SETAY_NS_PORT", "9090")
	input := `{
		title = "My App";
		server = {
			host = ${SETAY_NS_HOST};
			port = ${SETAY_NS_PORT}
		}
	}`
	var cfg VarNestedOuter
	if err := setay.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg.Server.Host != "nested-host" {
		t.Errorf("Server.Host = %q, want %q", cfg.Server.Host, "nested-host")
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("Server.Port = %d, want 9090", cfg.Server.Port)
	}
}

// TestVarInNestedStructWithFallback: fallback value in a nested struct field.
func TestVarInNestedStructWithFallback(t *testing.T) {
	os.Unsetenv("SETAY_NS2_PORT")
	input := `{
		title = "Test";
		server = {
			host = "localhost";
			port = ${SETAY_NS2_PORT ?: 8080}
		}
	}`
	var cfg VarNestedOuter
	if err := setay.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("Server.Port = %d, want 8080", cfg.Server.Port)
	}
}

// =====================================================================
//  Category 5: Single-quoted strings — NO interpolation
// =====================================================================

// TestVarNoInterpolationSingleQuoted: ${VAR} inside '' is a literal string.
func TestVarNoInterpolationSingleQuoted(t *testing.T) {
	// Even if the variable is set, single-quoted strings must NOT expand it.
	t.Setenv("SETAY_SQ_VAR", "should-not-appear")
	input := `{ str-val = '${SETAY_SQ_VAR}' }`
	var cfg VarTypedConfig
	if err := setay.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg.StrVal != "${SETAY_SQ_VAR}" {
		t.Errorf("StrVal = %q, want literal %q (no interpolation in single-quoted strings)", cfg.StrVal, "${SETAY_SQ_VAR}")
	}
}

// TestVarNoInterpolationSingleQuotedDollar: bare $ inside '' is also literal.
func TestVarNoInterpolationSingleQuotedDollar(t *testing.T) {
	input := `{ str-val = '$100 off' }`
	var cfg VarTypedConfig
	if err := setay.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg.StrVal != "$100 off" {
		t.Errorf("StrVal = %q, want %q", cfg.StrVal, "$100 off")
	}
}

// =====================================================================
//  Category 6: Edge cases
// =====================================================================

// TestVarEmptyEnvVar: env var is set to empty string ("") → found, empty value.
func TestVarEmptyEnvVar(t *testing.T) {
	t.Setenv("SETAY_EMPTY_VAR", "")
	// The variable IS defined (os.LookupEnv returns ok=true), so fallback must NOT be used.
	input := `{ str-val = ${SETAY_EMPTY_VAR ?: "fallback-not-used"} }`
	var cfg VarTypedConfig
	if err := setay.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg.StrVal != "" {
		t.Errorf("StrVal = %q, want empty string (env var is set but empty)", cfg.StrVal)
	}
}

// TestVarWhitespaceInsideBraces: ${ VAR } with spaces is valid.
func TestVarWhitespaceInsideBraces(t *testing.T) {
	t.Setenv("SETAY_WS_VAR", "ws-value")
	// The grammar allows SetaySpacing around varName inside ${ ... }
	input := `{ str-val = ${  SETAY_WS_VAR  } }`
	var cfg VarTypedConfig
	if err := setay.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg.StrVal != "ws-value" {
		t.Errorf("StrVal = %q, want %q", cfg.StrVal, "ws-value")
	}
}

// TestVarNameWithUnderscoreAndDigits: variable names like SETAY_VAR_123 are valid.
func TestVarNameWithUnderscoreAndDigits(t *testing.T) {
	t.Setenv("SETAY_VAR_123", "underscore-digits")
	input := `{ str-val = ${SETAY_VAR_123} }`
	var cfg VarTypedConfig
	if err := setay.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg.StrVal != "underscore-digits" {
		t.Errorf("StrVal = %q, want %q", cfg.StrVal, "underscore-digits")
	}
}

// TestVarLowercaseName: lowercase variable names are valid.
func TestVarLowercaseName(t *testing.T) {
	t.Setenv("myvar", "lowercase")
	input := `{ str-val = ${myvar} }`
	var cfg VarTypedConfig
	if err := setay.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg.StrVal != "lowercase" {
		t.Errorf("StrVal = %q, want %q", cfg.StrVal, "lowercase")
	}
}

// TestVarMultipleFieldsSameDocument: multiple ${} in one document.
func TestVarMultipleFieldsSameDocument(t *testing.T) {
	t.Setenv("SETAY_MF_HOST", "multi-host")
	t.Setenv("SETAY_MF_PORT", "5432")
	os.Unsetenv("SETAY_MF_MISSING")
	input := `{
		str-val = ${SETAY_MF_HOST};
		int-val = ${SETAY_MF_PORT};
		float-val = ${SETAY_MF_MISSING ?: 9.99};
		bool-val = ${SETAY_MF_MISSING ?: "true"}
	}`
	var cfg VarTypedConfig
	if err := setay.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg.StrVal != "multi-host" {
		t.Errorf("StrVal = %q, want %q", cfg.StrVal, "multi-host")
	}
	if cfg.IntVal != 5432 {
		t.Errorf("IntVal = %d, want 5432", cfg.IntVal)
	}
	if cfg.FloatVal != 9.99 {
		t.Errorf("FloatVal = %f, want 9.99", cfg.FloatVal)
	}
	if !cfg.BoolVal {
		t.Error("BoolVal = false, want true")
	}
}

// =====================================================================
//  Category 7: String interpolation edge cases
// =====================================================================

// TestVarInterpolationOnlyVars: string composed entirely of variable refs.
func TestVarInterpolationOnlyVars(t *testing.T) {
	t.Setenv("SETAY_IO_A", "foo")
	t.Setenv("SETAY_IO_B", "bar")
	input := `{ str-val = "${SETAY_IO_A}${SETAY_IO_B}" }`
	var cfg VarTypedConfig
	if err := setay.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg.StrVal != "foobar" {
		t.Errorf("StrVal = %q, want %q", cfg.StrVal, "foobar")
	}
}

// TestVarInterpolationLeadingText: "start ${VAR}" with text before the variable.
func TestVarInterpolationLeadingText(t *testing.T) {
	t.Setenv("SETAY_IL_VAR", "world")
	input := `{ str-val = "hello ${SETAY_IL_VAR}" }`
	var cfg VarTypedConfig
	if err := setay.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg.StrVal != "hello world" {
		t.Errorf("StrVal = %q, want %q", cfg.StrVal, "hello world")
	}
}

// TestVarInterpolationTrailingText: "${VAR} end" with text after the variable.
func TestVarInterpolationTrailingText(t *testing.T) {
	t.Setenv("SETAY_IT_VAR", "hello")
	input := `{ str-val = "${SETAY_IT_VAR} end" }`
	var cfg VarTypedConfig
	if err := setay.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg.StrVal != "hello end" {
		t.Errorf("StrVal = %q, want %q", cfg.StrVal, "hello end")
	}
}

// TestVarInterpolationMultipleDollarSigns: string with multiple bare $ signs.
func TestVarInterpolationMultipleDollarSigns(t *testing.T) {
	input := `{ str-val = "$10 + $20 = $30" }`
	var cfg VarTypedConfig
	if err := setay.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg.StrVal != "$10 + $20 = $30" {
		t.Errorf("StrVal = %q, want %q", cfg.StrVal, "$10 + $20 = $30")
	}
}

// TestVarInterpolationMixedLiteralsAndVars: "name=${VAR}, port=$literal".
func TestVarInterpolationMixedLiteralsAndVars(t *testing.T) {
	t.Setenv("SETAY_MIX_VAR", "Bob")
	input := `{ str-val = "name=${SETAY_MIX_VAR}, price=$99" }`
	var cfg VarTypedConfig
	if err := setay.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg.StrVal != "name=Bob, price=$99" {
		t.Errorf("StrVal = %q, want %q", cfg.StrVal, "name=Bob, price=$99")
	}
}

// =====================================================================
//  Category 8: Resolver verification
// =====================================================================

// TestVarResolverReceivesExactName: verifies the resolver is called with the exact variable name.
func TestVarResolverReceivesExactName(t *testing.T) {
	var receivedNames []string
	setay.RegisterVariableResolver(func(name string) (string, bool, error) {
		receivedNames = append(receivedNames, name)
		return "value-of-" + name, true, nil
	})
	defer setay.RegisterVariableResolver(nil)

	// Use two string fields to avoid type-coercion issues.
	input := `{ str-val = ${EXACT_NAME_A}; any-val = ${EXACT_NAME_B} }`
	var cfg VarTypedConfig
	if err := setay.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if len(receivedNames) != 2 {
		t.Fatalf("resolver called %d times, want 2", len(receivedNames))
	}
	if receivedNames[0] != "EXACT_NAME_A" {
		t.Errorf("receivedNames[0] = %q, want %q", receivedNames[0], "EXACT_NAME_A")
	}
	if receivedNames[1] != "EXACT_NAME_B" {
		t.Errorf("receivedNames[1] = %q, want %q", receivedNames[1], "EXACT_NAME_B")
	}
	if cfg.StrVal != "value-of-EXACT_NAME_A" {
		t.Errorf("StrVal = %q, want %q", cfg.StrVal, "value-of-EXACT_NAME_A")
	}
}

// TestVarResolverNotCalledWhenNil: when no resolver is registered, resolver is not called.
func TestVarResolverNotCalledWhenNil(t *testing.T) {
	// Ensure resolver is nil.
	setay.RegisterVariableResolver(nil)

	t.Setenv("SETAY_NO_RESOLVER", "env-only")
	input := `{ str-val = ${SETAY_NO_RESOLVER} }`
	var cfg VarTypedConfig
	if err := setay.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg.StrVal != "env-only" {
		t.Errorf("StrVal = %q, want %q", cfg.StrVal, "env-only")
	}
}

// TestVarResolverFallbackNotCalledWhenVarFound: fallback is not evaluated if var resolves.
func TestVarResolverFallbackNotCalledWhenVarFound(t *testing.T) {
	// The fallback variable SETAY_FB_SECONDARY is defined — but it should not be consulted
	// because the primary variable resolves successfully.
	t.Setenv("SETAY_FB_PRIMARY", "primary-value")
	t.Setenv("SETAY_FB_SECONDARY", "secondary-value")

	var resolved []string
	setay.RegisterVariableResolver(func(name string) (string, bool, error) {
		resolved = append(resolved, name)
		if name == "SETAY_FB_PRIMARY" {
			return "primary-value", true, nil
		}
		// Should never be called for secondary.
		return "", false, nil
	})
	defer setay.RegisterVariableResolver(nil)

	input := `{ str-val = ${SETAY_FB_PRIMARY ?: SETAY_FB_SECONDARY} }`
	var cfg VarTypedConfig
	if err := setay.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg.StrVal != "primary-value" {
		t.Errorf("StrVal = %q, want %q", cfg.StrVal, "primary-value")
	}
	// Only the primary should have been resolved.
	if len(resolved) != 1 || resolved[0] != "SETAY_FB_PRIMARY" {
		t.Errorf("resolved = %v, want [SETAY_FB_PRIMARY]", resolved)
	}
}

// =====================================================================
//  Category 9: Error message quality
// =====================================================================

// TestVarErrorContainsVarName: error for undefined var includes the variable name.
func TestVarErrorContainsVarName(t *testing.T) {
	os.Unsetenv("SETAY_ERR_UNIQUE_XYZ987")
	input := `{ str-val = ${SETAY_ERR_UNIQUE_XYZ987} }`
	var cfg VarTypedConfig
	err := setay.Unmarshal([]byte(input), &cfg)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "SETAY_ERR_UNIQUE_XYZ987") {
		t.Errorf("Error = %q — should contain the variable name", err.Error())
	}
}

// TestVarErrorContainsVarNameInFallback: error for unresolved fallback chain includes var name.
func TestVarErrorContainsVarNameInFallback(t *testing.T) {
	os.Unsetenv("SETAY_ERR_CHAIN_A")
	os.Unsetenv("SETAY_ERR_CHAIN_B")
	input := `{ str-val = ${SETAY_ERR_CHAIN_A ?: SETAY_ERR_CHAIN_B} }`
	var cfg VarTypedConfig
	err := setay.Unmarshal([]byte(input), &cfg)
	if err == nil {
		t.Fatal("Expected error for unresolved fallback chain, got nil")
	}
	// Error should mention at least one of the variable names.
	if !strings.Contains(err.Error(), "SETAY_ERR_CHAIN") {
		t.Errorf("Error = %q — should contain a chain variable name", err.Error())
	}
}

// TestVarResolverErrorIsWrapped: resolver error is wrapped and propagated.
func TestVarResolverErrorIsWrapped(t *testing.T) {
	sentinelErr := fmt.Errorf("sentinel-database-error")
	setay.RegisterVariableResolver(func(name string) (string, bool, error) {
		return "", false, sentinelErr
	})
	defer setay.RegisterVariableResolver(nil)

	input := `{ str-val = ${ANY_VAR} }`
	var cfg VarTypedConfig
	err := setay.Unmarshal([]byte(input), &cfg)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "sentinel-database-error") {
		t.Errorf("Error = %q — should contain the sentinel error message", err.Error())
	}
}

// =====================================================================
//  Category 10: Interaction with normal literal values
// =====================================================================

// TestVarMixedWithNormalValues: dict with both variable and literal values.
func TestVarMixedWithNormalValues(t *testing.T) {
	t.Setenv("SETAY_MIX_DB_HOST", "db.example.com")
	input := `{
		str-val   = ${SETAY_MIX_DB_HOST};
		int-val   = 5432;
		bool-val  = true;
		float-val = 1.5
	}`
	var cfg VarTypedConfig
	if err := setay.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg.StrVal != "db.example.com" {
		t.Errorf("StrVal = %q, want %q", cfg.StrVal, "db.example.com")
	}
	if cfg.IntVal != 5432 {
		t.Errorf("IntVal = %d, want 5432", cfg.IntVal)
	}
	if !cfg.BoolVal {
		t.Error("BoolVal = false, want true")
	}
	if cfg.FloatVal != 1.5 {
		t.Errorf("FloatVal = %f, want 1.5", cfg.FloatVal)
	}
}

// TestVarDoesNotAffectNonVarFields: unmarshaling without any variables still works.
func TestVarDoesNotAffectNonVarFields(t *testing.T) {
	setay.RegisterVariableResolver(func(name string) (string, bool, error) {
		// This resolver should never be called — no variable refs in input.
		return "", false, fmt.Errorf("resolver should not be called")
	})
	defer setay.RegisterVariableResolver(nil)

	input := `{
		str-val   = "plain string";
		int-val   = 100;
		bool-val  = false
	}`
	var cfg VarTypedConfig
	if err := setay.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg.StrVal != "plain string" {
		t.Errorf("StrVal = %q, want %q", cfg.StrVal, "plain string")
	}
	if cfg.IntVal != 100 {
		t.Errorf("IntVal = %d, want 100", cfg.IntVal)
	}
	if cfg.BoolVal {
		t.Error("BoolVal = true, want false")
	}
}
