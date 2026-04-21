package setay

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// Unmarshal parses setay-encoded data and stores the result in the value pointed to by v.
func Unmarshal(data []byte, v interface{}) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return fmt.Errorf("setay: Unmarshal requires a non-nil pointer")
	}

	source := string(data)
	doc, err := Parse(source)
	if err != nil {
		return fmt.Errorf("setay: %w", err)
	}

	// Verify all input consumed
	runes := []rune(source)
	start := int(doc.Authority.StartedAt)
	length := int(doc.Authority.Length)
	if start+length != len(runes) {
		return fmt.Errorf("setay: parse incomplete (consumed %d of %d characters)", start+length, len(runes))
	}

	u := &unmarshaler{source: runes}
	return u.unmarshalDict(doc.Dict, rv.Elem())
}

// UnmarshalFile reads a setay file and parses it into v.
func UnmarshalFile(filename string, v interface{}) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	return Unmarshal(data, v)
}

type unmarshaler struct {
	source []rune
}

// textOf extracts the source text for a node.
func (u *unmarshaler) textOf(auth *Authority) string {
	start := int(auth.StartedAt)
	end := start + int(auth.Length)
	if end > len(u.source) {
		end = len(u.source)
	}
	return string(u.source[start:end])
}

// unmarshalDict populates a struct or map from a SetayDict node.
func (u *unmarshaler) unmarshalDict(dict *DefSetayDict, target reflect.Value) error {
	// Unwrap pointer
	for target.Kind() == reflect.Ptr {
		if target.IsNil() {
			target.Set(reflect.New(target.Type().Elem()))
		}
		target = target.Elem()
	}

	if target.Kind() == reflect.Map {
		return u.unmarshalDictToMap(dict, target)
	}
	if target.Kind() == reflect.Struct {
		return u.unmarshalDictToStruct(dict, target)
	}
	// Also support map[string]interface{} via interface{}
	if target.Kind() == reflect.Interface {
		m := make(map[string]interface{})
		mapVal := reflect.ValueOf(m)
		if err := u.unmarshalDictToMap(dict, mapVal); err != nil {
			return err
		}
		target.Set(mapVal)
		return nil
	}

	return fmt.Errorf("setay: cannot unmarshal dict into %s", target.Type())
}

func (u *unmarshaler) unmarshalDictToStruct(dict *DefSetayDict, target reflect.Value) error {
	if len(dict.Entries) == 0 {
		return nil
	}

	entries := dict.Entries[0]
	fieldMap := buildFieldMap(target.Type())

	// Process first entry
	if err := u.setStructField(entries.First, target, fieldMap); err != nil {
		return err
	}
	// Process rest
	for _, sep := range entries.Rest {
		if err := u.setStructField(sep.Entry, target, fieldMap); err != nil {
			return err
		}
	}
	return nil
}

func (u *unmarshaler) unmarshalDictToMap(dict *DefSetayDict, target reflect.Value) error {
	if target.IsNil() {
		target.Set(reflect.MakeMap(target.Type()))
	}

	if len(dict.Entries) == 0 {
		return nil
	}

	entries := dict.Entries[0]
	valType := target.Type().Elem()

	if err := u.setMapEntry(entries.First, target, valType); err != nil {
		return err
	}
	for _, sep := range entries.Rest {
		if err := u.setMapEntry(sep.Entry, target, valType); err != nil {
			return err
		}
	}
	return nil
}

// buildFieldMap creates a mapping from setay key name → struct field index.
func buildFieldMap(t reflect.Type) map[string]int {
	m := make(map[string]int)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		opts := parseTag(field)
		if opts.skip {
			continue
		}
		m[opts.name] = i
		// Also map lowercase field name for case-insensitive fallback
		lower := strings.ToLower(field.Name)
		if _, exists := m[lower]; !exists {
			m[lower] = i
		}
	}
	return m
}

func (u *unmarshaler) setStructField(entry *DefSetayDictEntry, target reflect.Value, fieldMap map[string]int) error {
	keyText := u.extractKeyText(entry.Key)

	idx, ok := fieldMap[keyText]
	if !ok {
		// Try case-insensitive
		idx, ok = fieldMap[strings.ToLower(keyText)]
	}
	if !ok {
		// Unknown field — skip silently (like encoding/json)
		return nil
	}

	fieldVal := target.Field(idx)
	return u.unmarshalValue(entry.Value, fieldVal)
}

func (u *unmarshaler) setMapEntry(entry *DefSetayDictEntry, target reflect.Value, valType reflect.Type) error {
	keyText := u.extractKeyText(entry.Key)

	val := reflect.New(valType).Elem()
	if err := u.unmarshalValue(entry.Value, val); err != nil {
		return err
	}
	target.SetMapIndex(reflect.ValueOf(keyText), val)
	return nil
}

// extractKeyText gets the key as a plain string.
// Variable resolution errors inside a DQ string key are silently ignored and "" is
// returned. Keys are ordinarily plain literals, so this is acceptable — however, an
// entry whose key contains an undefined variable will be silently skipped.
func (u *unmarshaler) extractKeyText(key *DefSetayDictKey) string {
	inner := key.AnonymousField1
	switch v := inner.(type) {
	case *DefSetayBareKey:
		return u.textOf(v.GetAuthority())
	case *DefSetayString:
		// Ignore variable resolution errors in string keys — keys are ordinarily literals.
		s, _ := u.decodeString(v)
		return s
	default:
		return u.textOf(key.GetAuthority())
	}
}

// unmarshalValue converts a SetayValue node into a Go reflect.Value.
func (u *unmarshaler) unmarshalValue(val *DefSetayValue, target reflect.Value) error {
	// Unwrap pointer: allocate if nil
	for target.Kind() == reflect.Ptr {
		if target.IsNil() {
			target.Set(reflect.New(target.Type().Elem()))
		}
		target = target.Elem()
	}

	inner := val.AnonymousField1

	switch v := inner.(type) {
	case *DefSetayVarRef:
		// Resolve the variable and coerce the result to the target type.
		resolved, err := u.resolveVarRef(v)
		if err != nil {
			return err
		}
		return u.setFromString(target, resolved)
	case *DefSetayNull:
		return u.setNull(target)
	case *DefSetayTrue:
		return u.setBool(target, true)
	case *DefSetayFalse:
		return u.setBool(target, false)
	case *DefSetayString:
		s, err := u.decodeString(v)
		if err != nil {
			return err
		}
		return u.setString(target, s)
	case *DefSetayNumber:
		numText := u.textOf(v.GetAuthority())
		return u.setNumber(target, numText)
	case *DefSetaySet:
		return u.unmarshalSet(v, target)
	case *DefSetayDict:
		return u.unmarshalDict(v, target)
	case *DefSetayList:
		return u.unmarshalList(v, target)
	case *DefSetayUtcTs:
		return u.setUtcTs(v, target)
	default:
		return fmt.Errorf("setay: unsupported value type")
	}
}

func (u *unmarshaler) setNull(target reflect.Value) error {
	switch target.Kind() {
	case reflect.Ptr, reflect.Interface, reflect.Slice, reflect.Map:
		target.Set(reflect.Zero(target.Type()))
	default:
		target.Set(reflect.Zero(target.Type()))
	}
	return nil
}

func (u *unmarshaler) setBool(target reflect.Value, b bool) error {
	switch target.Kind() {
	case reflect.Bool:
		target.SetBool(b)
	case reflect.Interface:
		target.Set(reflect.ValueOf(b))
	default:
		return fmt.Errorf("setay: cannot set bool into %s", target.Type())
	}
	return nil
}

func (u *unmarshaler) setString(target reflect.Value, s string) error {
	switch target.Kind() {
	case reflect.String:
		target.SetString(s)
	case reflect.Interface:
		target.Set(reflect.ValueOf(s))
	default:
		return fmt.Errorf("setay: cannot set string into %s", target.Type())
	}
	return nil
}

func (u *unmarshaler) setNumber(target reflect.Value, text string) error {
	text = strings.TrimSpace(text)

	switch target.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := parseInteger(text)
		if err != nil {
			return fmt.Errorf("setay: %w", err)
		}
		target.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := parseInteger(text)
		if err != nil {
			return fmt.Errorf("setay: %w", err)
		}
		target.SetUint(uint64(n))
	case reflect.Float32, reflect.Float64:
		f, err := parseFloat(text)
		if err != nil {
			return fmt.Errorf("setay: %w", err)
		}
		target.SetFloat(f)
	case reflect.Interface:
		// Determine type: if contains '.', 'e', 'E' → float, else int
		if strings.ContainsAny(text, ".eE") {
			f, err := parseFloat(text)
			if err != nil {
				return fmt.Errorf("setay: %w", err)
			}
			target.Set(reflect.ValueOf(f))
		} else {
			n, err := parseInteger(text)
			if err != nil {
				return fmt.Errorf("setay: %w", err)
			}
			target.Set(reflect.ValueOf(n))
		}
	default:
		return fmt.Errorf("setay: cannot set number into %s", target.Type())
	}
	return nil
}

func (u *unmarshaler) unmarshalList(list *DefSetayList, target reflect.Value) error {
	if target.Kind() == reflect.Interface {
		// Decode into []interface{}
		var result []interface{}
		if len(list.Elements) > 0 {
			elements := list.Elements[0]
			values := collectValues(elements)
			result = make([]interface{}, len(values))
			for i, val := range values {
				var elem interface{}
				elemVal := reflect.ValueOf(&elem).Elem()
				if err := u.unmarshalValue(val, elemVal); err != nil {
					return err
				}
				result[i] = elem
			}
		}
		target.Set(reflect.ValueOf(result))
		return nil
	}

	if target.Kind() != reflect.Slice {
		return fmt.Errorf("setay: cannot unmarshal list into %s", target.Type())
	}

	if len(list.Elements) == 0 {
		target.Set(reflect.MakeSlice(target.Type(), 0, 0))
		return nil
	}

	elements := list.Elements[0]
	values := collectValues(elements)
	slice := reflect.MakeSlice(target.Type(), len(values), len(values))
	for i, val := range values {
		if err := u.unmarshalValue(val, slice.Index(i)); err != nil {
			return err
		}
	}
	target.Set(slice)
	return nil
}

func (u *unmarshaler) setUtcTs(utcts *DefSetayUtcTs, target reflect.Value) error {
	// Extract the string value inside UtcTs(...)
	s, err := u.decodeString(utcts.Value)
	if err != nil {
		return fmt.Errorf("setay: UtcTs variable resolution: %w", err)
	}

	// Try parsing various time formats
	formats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05Z",
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02T15:04:05.999999999",
		"2006-01-02 15:04:05.999999999Z",
		"2006-01-02T15:04:05.999999999Z",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02T15:04:05.999999999-07:00",
		"2006-01-02 15:04:05+07:00",
		"2006-01-02T15:04:05+07:00",
	}

	var t time.Time
	var parseErr error
	for _, format := range formats {
		t, parseErr = time.Parse(format, s)
		if parseErr == nil {
			break
		}
	}
	if parseErr != nil {
		return fmt.Errorf("setay: cannot parse UtcTs %q: %w", s, parseErr)
	}

	if target.Kind() == reflect.Interface {
		target.Set(reflect.ValueOf(t))
		return nil
	}
	if target.Type() == reflect.TypeOf(time.Time{}) {
		target.Set(reflect.ValueOf(t))
		return nil
	}
	if target.Kind() == reflect.String {
		target.SetString(s)
		return nil
	}

	// Check for wantai timestamp types (types implementing ToTime() time.Time).
	// Wantai types are named types backed by integer primitives (int64, int32, uint32).
	// We convert time.Time to the appropriate integer representation based on the type name.
	toTimerType := reflect.TypeOf((*toTimer)(nil)).Elem()
	if reflect.PointerTo(target.Type()).Implements(toTimerType) || target.Type().Implements(toTimerType) {
		return u.setWantaiTs(t, target)
	}

	return fmt.Errorf("setay: cannot set UtcTs into %s", target.Type())
}

// setWantaiTs converts a time.Time to a wantai timestamp type and sets it on target.
// The conversion is based on the type name to determine the appropriate precision.
func (u *unmarshaler) setWantaiTs(t time.Time, target reflect.Value) error {
	typeName := target.Type().Name()
	tUTC := t.UTC()

	var intVal int64
	switch typeName {
	case "UtcNanoTs":
		intVal = tUTC.UnixNano()
	case "UtcMicroTs":
		intVal = tUTC.UnixMicro()
	case "UtcMilliTs":
		intVal = tUTC.UnixMilli()
	case "UtcSecTsS32", "UtcSecTsU32", "UtcSecTsS32Ep2k", "UtcSecTsU32Ep2k":
		unixSec := tUTC.Unix()
		if typeName == "UtcSecTsS32Ep2k" || typeName == "UtcSecTsU32Ep2k" {
			// Epoch 2000: subtract seconds between 1970-01-01 and 2000-01-01
			const epoch2k = 946684800 // 2000-01-01T00:00:00Z in Unix seconds
			unixSec -= epoch2k
		}
		intVal = unixSec
	default:
		// Fallback: try seconds-level precision
		intVal = tUTC.Unix()
	}

	switch target.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		target.SetInt(intVal)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		target.SetUint(uint64(intVal))
	default:
		return fmt.Errorf("setay: cannot set UtcTs into wantai type %s (kind %s)", target.Type(), target.Kind())
	}
	return nil
}

// decodeString extracts the unescaped string content from a SetayString node.
// An error is returned only when variable resolution inside a DQ string fails.
func (u *unmarshaler) decodeString(str *DefSetayString) (string, error) {
	inner := str.AnonymousField1
	switch v := inner.(type) {
	case *DefSetayDqString:
		return u.decodeStringContent(v.Content)
	case *DefSetaySqString:
		return u.decodeSqStringContent(v.Content), nil
	default:
		// Fallback: strip quotes from raw text
		raw := u.textOf(str.GetAuthority())
		if len(raw) >= 2 {
			return raw[1 : len(raw)-1], nil
		}
		return raw, nil
	}
}

func (u *unmarshaler) decodeStringContent(contents []*DefSetayDqStringContent) (string, error) {
	var sb strings.Builder
	for _, c := range contents {
		inner := c.AnonymousField1
		switch v := inner.(type) {
		case *DefSetayVarRef:
			// Variable interpolation inside a DQ string — errors propagate to the caller.
			resolved, err := u.resolveVarRef(v)
			if err != nil {
				return "", err
			}
			sb.WriteString(resolved)
		case *DefSetayEscapeSequence:
			sb.WriteString(u.decodeEscape(v))
		case *DefSetayDqDollarChar:
			// Bare '$' not followed by '{': emit it literally.
			sb.WriteString("$")
		case *DefSetayDqNormalChar:
			sb.WriteString(u.textOf(v.GetAuthority()))
		default:
			sb.WriteString(u.textOf(c.GetAuthority()))
		}
	}
	return sb.String(), nil
}

func (u *unmarshaler) decodeSqStringContent(contents []*DefSetaySqStringContent) string {
	var sb strings.Builder
	for _, c := range contents {
		inner := c.AnonymousField1
		switch v := inner.(type) {
		case *DefSetayEscapeSequence:
			sb.WriteString(u.decodeEscape(v))
		case *DefSetaySqNormalChar:
			sb.WriteString(u.textOf(v.GetAuthority()))
		default:
			sb.WriteString(u.textOf(c.GetAuthority()))
		}
	}
	return sb.String()
}

func (u *unmarshaler) decodeEscape(esc *DefSetayEscapeSequence) string {
	escText := u.textOf(esc.GetAuthority())
	if len(escText) < 2 {
		return escText
	}
	ch := escText[1]
	switch ch {
	case 't':
		return "\t"
	case 'r':
		return "\r"
	case 'n':
		return "\n"
	case '0':
		return "\x00"
	case '\\':
		return "\\"
	case '"':
		return "\""
	case '\'':
		return "'"
	case 'x':
		// ParseInt cannot fail here: the PEG grammar requires exactly 2 hex digits.
		if len(escText) == 4 {
			n, _ := strconv.ParseInt(escText[2:4], 16, 32)
			return string(rune(n))
		}
	case 'u':
		// ParseInt cannot fail here: the PEG grammar requires exactly 4 hex digits.
		if len(escText) == 6 {
			n, _ := strconv.ParseInt(escText[2:6], 16, 32)
			return string(rune(n))
		}
	case 'U':
		// ParseInt cannot fail here: the PEG grammar requires exactly 8 hex digits.
		if len(escText) == 10 {
			n, _ := strconv.ParseInt(escText[2:10], 16, 32)
			return string(rune(n))
		}
	}
	return escText
}

// unmarshalSet populates a map[K]struct{} from a SetaySet node.
func (u *unmarshaler) unmarshalSet(set *DefSetaySet, target reflect.Value) error {
	for target.Kind() == reflect.Ptr {
		if target.IsNil() {
			target.Set(reflect.New(target.Type().Elem()))
		}
		target = target.Elem()
	}

	if target.Kind() != reflect.Map {
		return fmt.Errorf("setay: cannot unmarshal set into %s", target.Type())
	}
	emptyStruct := reflect.TypeOf(struct{}{})
	if target.Type().Elem() != emptyStruct {
		return fmt.Errorf("setay: cannot unmarshal set into %s (value type must be struct{})", target.Type())
	}

	if target.IsNil() {
		target.Set(reflect.MakeMap(target.Type()))
	}

	switch v := set.Body.AnonymousField1.(type) {
	case *DefSetaySetEmpty:
		// empty set — map already initialized, nothing to do
		return nil
	case *DefSetaySetEntries:
		keyType := target.Type().Key()
		structVal := reflect.ValueOf(struct{}{})
		for _, entry := range collectSetEntries(v) {
			keyVal := reflect.New(keyType).Elem()
			if err := u.unmarshalSetKey(entry.Key, keyVal); err != nil {
				return err
			}
			target.SetMapIndex(keyVal, structVal)
		}
		return nil
	default:
		return fmt.Errorf("setay: unexpected set body type")
	}
}

// unmarshalSetKey converts a SetaySetKey node into a Go reflect.Value.
func (u *unmarshaler) unmarshalSetKey(key *DefSetaySetKey, target reflect.Value) error {
	for target.Kind() == reflect.Ptr {
		if target.IsNil() {
			target.Set(reflect.New(target.Type().Elem()))
		}
		target = target.Elem()
	}

	switch v := key.AnonymousField1.(type) {
	case *DefSetayVarRef:
		resolved, err := u.resolveVarRef(v)
		if err != nil {
			return err
		}
		return u.setFromString(target, resolved)
	case *DefSetayNull:
		return u.setNull(target)
	case *DefSetayTrue:
		return u.setBool(target, true)
	case *DefSetayFalse:
		return u.setBool(target, false)
	case *DefSetayString:
		s, err := u.decodeString(v)
		if err != nil {
			return err
		}
		return u.setString(target, s)
	case *DefSetayNumber:
		numText := u.textOf(v.GetAuthority())
		return u.setNumber(target, numText)
	case *DefSetayUtcTs:
		return u.setUtcTs(v, target)
	default:
		return fmt.Errorf("setay: unsupported set key type")
	}
}

// collectSetEntries gathers all entries from SetaySetEntries.
func collectSetEntries(entries *DefSetaySetEntries) []*DefSetaySetEntry {
	result := []*DefSetaySetEntry{entries.First}
	for _, e := range entries.Rest {
		result = append(result, e)
	}
	return result
}

// collectValues extracts all values from ListElements.
func collectValues(elements *DefSetayListElements) []*DefSetayValue {
	result := []*DefSetayValue{elements.First}
	for _, sep := range elements.Rest {
		result = append(result, sep.Value)
	}
	return result
}

// resolveVarRef resolves a ${VAR ?: fallback} reference.
// It walks the fallback chain recursively until a value is found or all
// options are exhausted, in which case it returns an error.
func (u *unmarshaler) resolveVarRef(ref *DefSetayVarRef) (string, error) {
	varName := u.textOf(ref.Expr.VarName.GetAuthority())

	val, ok, err := resolveVar(varName)
	if err != nil {
		return "", fmt.Errorf("setay: variable resolver error for %q: %w", varName, err)
	}
	if ok {
		return val, nil
	}

	// Not found — try fallback chain.
	if len(ref.Expr.Fallback) > 0 {
		return u.resolveFallback(ref.Expr.Fallback[0])
	}

	return "", fmt.Errorf("setay: variable %q is not defined and has no fallback", varName)
}

// resolveFallback evaluates a single fallback node (and recurses into .Next).
func (u *unmarshaler) resolveFallback(fb *DefSetayVarFallback) (string, error) {
	v := fb.Value.AnonymousField1
	switch node := v.(type) {
	case *DefSetayVarRef:
		// Another variable reference as fallback — try it.
		varName := u.textOf(node.Expr.VarName.GetAuthority())
		val, ok, err := resolveVar(varName)
		if err != nil {
			return "", fmt.Errorf("setay: variable resolver error for %q: %w", varName, err)
		}
		if ok {
			return val, nil
		}
		// Not found — continue the chain inside this VarRef's own fallback.
		if len(node.Expr.Fallback) > 0 {
			result, err2 := u.resolveFallback(node.Expr.Fallback[0])
			if err2 == nil {
				return result, nil
			}
			// All errors from the recursive call propagate upward.
			// (undefined variable / fallback exhausted / resolver error — none are excluded)
			return "", err2
		}
		// Still not found — try the sibling Next chain.
		if len(fb.Next) > 0 {
			return u.resolveFallback(fb.Next[0])
		}
		return "", fmt.Errorf("setay: variable %q is not defined and has no more fallbacks", varName)
	case *DefSetayVarName:
		// Bare variable name as fallback: ${A ?: B ?: "last"} — B is resolved as a variable.
		varName := u.textOf(node.GetAuthority())
		val, ok, err := resolveVar(varName)
		if err != nil {
			return "", fmt.Errorf("setay: variable resolver error for %q: %w", varName, err)
		}
		if ok {
			return val, nil
		}
		// Variable B not found — continue to next fallback.
		if len(fb.Next) > 0 {
			return u.resolveFallback(fb.Next[0])
		}
		return "", fmt.Errorf("setay: variable %q is not defined and has no more fallbacks", varName)
	case *DefSetayString:
		// Literal string fallback.
		s, err := u.decodeString(node)
		if err != nil {
			return "", err
		}
		return s, nil
	case *DefSetayNumber:
		// Literal number fallback — return as-is string.
		return u.textOf(node.GetAuthority()), nil
	case *DefSetayTrue:
		// Literal boolean true fallback — return "true" so setFromString can coerce to bool.
		return "true", nil
	case *DefSetayFalse:
		// Literal boolean false fallback — return "false" so setFromString can coerce to bool.
		return "false", nil
	default:
		return u.textOf(fb.Value.GetAuthority()), nil
	}
}

// setFromString coerces a resolved variable string value to the target reflect.Value.
// Strings go to string/interface; numbers are parsed when the target is numeric.
func (u *unmarshaler) setFromString(target reflect.Value, s string) error {
	switch target.Kind() {
	case reflect.String:
		target.SetString(s)
		return nil
	case reflect.Interface:
		target.Set(reflect.ValueOf(s))
		return nil
	case reflect.Bool:
		switch strings.ToLower(strings.TrimSpace(s)) {
		case "true", "1", "yes":
			target.SetBool(true)
		case "false", "0", "no", "":
			target.SetBool(false)
		default:
			return fmt.Errorf("setay: cannot parse %q as bool", s)
		}
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return u.setNumber(target, s)
	default:
		return fmt.Errorf("setay: cannot set variable (string %q) into %s", s, target.Type())
	}
}

// parseInteger parses an integer string, handling prefixes.
func parseInteger(s string) (int64, error) {
	neg := false
	if strings.HasPrefix(s, "-") {
		neg = true
		s = s[1:]
	}

	var n int64
	var err error

	switch {
	case strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X"):
		n, err = strconv.ParseInt(s[2:], 16, 64)
	case strings.HasPrefix(s, "0b") || strings.HasPrefix(s, "0B"):
		n, err = strconv.ParseInt(s[2:], 2, 64)
	case strings.HasPrefix(s, "0o") || strings.HasPrefix(s, "0O"):
		n, err = strconv.ParseInt(s[2:], 8, 64)
	case strings.HasPrefix(s, "0d") || strings.HasPrefix(s, "0D"):
		n, err = strconv.ParseInt(s[2:], 10, 64)
	default:
		n, err = strconv.ParseInt(s, 10, 64)
	}

	if err != nil {
		return 0, err
	}
	if neg {
		n = -n
	}
	return n, nil
}

// parseFloat parses a float string.
func parseFloat(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}
