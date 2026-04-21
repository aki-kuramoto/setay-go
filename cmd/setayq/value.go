package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	setay "github.com/aki-kuramoto/setay-go"
)

// valueKind represents the type of a setayq runtime value.
type valueKind int

const (
	kindNull valueKind = iota
	kindBool
	kindNumber
	kindString
	kindArray
	kindDict
	kindSet
	kindTimestamp
	kindVarRef // unresolved ${VAR} reference
)

// Value is the runtime representation of a setay value inside setayq.
type Value struct {
	kind valueKind

	boolVal   bool
	numText   string  // kept as text to preserve int/float distinction
	numFloat  float64 // parsed number
	strVal    string
	arrVal    []*Value
	dictKeys  []string            // ordered keys
	dictVals  map[string]*Value
	setKeys   []string
	tsVal     time.Time
	varName   string // for kindVarRef
	varFallback string
}

func (v *Value) typeName() string {
	switch v.kind {
	case kindNull:
		return "null"
	case kindBool:
		return "boolean"
	case kindNumber:
		return "number"
	case kindString:
		return "string"
	case kindArray:
		return "array"
	case kindDict:
		return "object"
	case kindSet:
		return "set"
	case kindTimestamp:
		return "timestamp"
	case kindVarRef:
		return "variable"
	default:
		return "unknown"
	}
}

// ---- AST → Value conversion ----

// astDictToValue converts a parsed SetayDict AST node into a runtime Value.
func astDictToValue(dict *setay.DefSetayDict, src []rune) *Value {
	v := &Value{
		kind:     kindDict,
		dictKeys: nil,
		dictVals: make(map[string]*Value),
	}
	if len(dict.Entries) == 0 {
		return v
	}
	entries := dict.Entries[0]
	addEntry := func(entry *setay.DefSetayDictEntry) {
		key := astKeyText(entry.Key, src)
		val := astValueToValue(entry.Value, src)
		if _, exists := v.dictVals[key]; !exists {
			v.dictKeys = append(v.dictKeys, key)
		}
		v.dictVals[key] = val
	}
	addEntry(entries.First)
	for _, sep := range entries.Rest {
		addEntry(sep.Entry)
	}
	return v
}

func astKeyText(key *setay.DefSetayDictKey, src []rune) string {
	inner := key.AnonymousField1
	switch v := inner.(type) {
	case *setay.DefSetayBareKey:
		return textOf(v.GetAuthority(), src)
	case *setay.DefSetayString:
		s, _ := astStringValue(v, src)
		return s
	default:
		return textOf(key.GetAuthority(), src)
	}
}

func astValueToValue(val *setay.DefSetayValue, src []rune) *Value {
	inner := val.AnonymousField1
	switch node := inner.(type) {
	case *setay.DefSetayNull:
		return &Value{kind: kindNull}
	case *setay.DefSetayTrue:
		return &Value{kind: kindBool, boolVal: true}
	case *setay.DefSetayFalse:
		return &Value{kind: kindBool, boolVal: false}
	case *setay.DefSetayString:
		s, _ := astStringValue(node, src)
		return &Value{kind: kindString, strVal: s}
	case *setay.DefSetayNumber:
		t := textOf(node.GetAuthority(), src)
		f, _ := parseNumber(t)
		return &Value{kind: kindNumber, numText: t, numFloat: f}
	case *setay.DefSetayDict:
		return astDictToValue(node, src)
	case *setay.DefSetayList:
		return astListToValue(node, src)
	case *setay.DefSetaySet:
		return astSetToValue(node, src)
	case *setay.DefSetayUtcTs:
		s, _ := astStringValue(node.Value, src)
		t := parseTimestamp(s)
		return &Value{kind: kindTimestamp, tsVal: t, strVal: s}
	case *setay.DefSetayVarRef:
		varName := textOf(node.Expr.VarName.GetAuthority(), src)
		// try resolving from environment (if --resolve-vars not explicitly called)
		val, ok, _ := setay.ResolveVar(varName)
		if ok {
			return &Value{kind: kindString, strVal: val}
		}
		return &Value{kind: kindVarRef, varName: varName}
	default:
		return &Value{kind: kindNull}
	}
}

func astListToValue(list *setay.DefSetayList, src []rune) *Value {
	v := &Value{kind: kindArray}
	if len(list.Elements) == 0 {
		return v
	}
	elements := list.Elements[0]
	add := func(val *setay.DefSetayValue) {
		v.arrVal = append(v.arrVal, astValueToValue(val, src))
	}
	add(elements.First)
	for _, sep := range elements.Rest {
		add(sep.Value)
	}
	return v
}

func astSetToValue(set *setay.DefSetaySet, src []rune) *Value {
	v := &Value{kind: kindSet}
	switch body := set.Body.AnonymousField1.(type) {
	case *setay.DefSetaySetEntries:
		// collect entries
		for _, entry := range collectSetEntries(body) {
			key := astSetKeyText(entry.Key, src)
			v.setKeys = append(v.setKeys, key)
		}
	}
	return v
}

func collectSetEntries(entries *setay.DefSetaySetEntries) []*setay.DefSetaySetEntry {
	result := []*setay.DefSetaySetEntry{entries.First}
	for _, e := range entries.Rest {
		result = append(result, e)
	}
	return result
}

func astSetKeyText(key *setay.DefSetaySetKey, src []rune) string {
	inner := key.AnonymousField1
	switch v := inner.(type) {
	case *setay.DefSetayVarRef:
		name := textOf(v.Expr.VarName.GetAuthority(), src)
		val, ok, _ := setay.ResolveVar(name)
		if ok {
			return val
		}
		return "${" + name + "}"
	case *setay.DefSetayNull:
		return "null"
	case *setay.DefSetayTrue:
		return "true"
	case *setay.DefSetayFalse:
		return "false"
	case *setay.DefSetayString:
		s, _ := astStringValue(v, src)
		return s
	case *setay.DefSetayNumber:
		return textOf(v.GetAuthority(), src)
	default:
		return textOf(key.GetAuthority(), src)
	}
}

func astStringValue(str *setay.DefSetayString, src []rune) (string, error) {
	inner := str.AnonymousField1
	switch v := inner.(type) {
	case *setay.DefSetayDqString:
		return decodeDqString(v, src), nil
	case *setay.DefSetaySqString:
		return decodeSqString(v, src), nil
	default:
		raw := textOf(str.GetAuthority(), src)
		if len(raw) >= 2 {
			return raw[1 : len(raw)-1], nil
		}
		return raw, nil
	}
}

func decodeDqString(str *setay.DefSetayDqString, src []rune) string {
	var sb strings.Builder
	for _, c := range str.Content {
		inner := c.AnonymousField1
		switch v := inner.(type) {
		case *setay.DefSetayVarRef:
			varName := textOf(v.Expr.VarName.GetAuthority(), src)
			val, ok, _ := setay.ResolveVar(varName)
			if ok {
				sb.WriteString(val)
			} else {
				// keep the raw ${VAR} text
				sb.WriteString(textOf(v.GetAuthority(), src))
			}
		case *setay.DefSetayEscapeSequence:
			sb.WriteString(decodeEscape(textOf(v.GetAuthority(), src)))
		default:
			sb.WriteString(textOf(c.GetAuthority(), src))
		}
	}
	return sb.String()
}

func decodeSqString(str *setay.DefSetaySqString, src []rune) string {
	var sb strings.Builder
	for _, c := range str.Content {
		inner := c.AnonymousField1
		switch v := inner.(type) {
		case *setay.DefSetayEscapeSequence:
			sb.WriteString(decodeEscape(textOf(v.GetAuthority(), src)))
		default:
			sb.WriteString(textOf(c.GetAuthority(), src))
		}
	}
	return sb.String()
}

func decodeEscape(esc string) string {
	if len(esc) < 2 {
		return esc
	}
	switch esc[1] {
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
		if len(esc) == 4 {
			n, _ := strconv.ParseInt(esc[2:4], 16, 32)
			return string(rune(n))
		}
	case 'u':
		if len(esc) == 6 {
			n, _ := strconv.ParseInt(esc[2:6], 16, 32)
			return string(rune(n))
		}
	case 'U':
		if len(esc) == 10 {
			n, _ := strconv.ParseInt(esc[2:10], 16, 32)
			return string(rune(n))
		}
	}
	return esc
}

func textOf(auth *setay.Authority, src []rune) string {
	start := int(auth.StartedAt)
	end := start + int(auth.Length)
	if end > len(src) {
		end = len(src)
	}
	return string(src[start:end])
}

func parseNumber(s string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(s), 64)
}

func parseTimestamp(s string) time.Time {
	formats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05Z",
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02T15:04:05-07:00",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// ---- Value utilities ----

// getField accesses a key in a dict or set.
func (v *Value) getField(key string) (*Value, error) {
	switch v.kind {
	case kindDict:
		if val, ok := v.dictVals[key]; ok {
			return val, nil
		}
		return &Value{kind: kindNull}, nil
	case kindSet:
		for _, k := range v.setKeys {
			if k == key {
				return &Value{kind: kindBool, boolVal: true}, nil
			}
		}
		return &Value{kind: kindBool, boolVal: false}, nil
	case kindNull:
		return &Value{kind: kindNull}, nil
	default:
		return nil, fmt.Errorf("null (null) has no field %q", key)
	}
}

// getIndex accesses an element in an array by index.
func (v *Value) getIndex(idx int) (*Value, error) {
	if v.kind != kindArray {
		return nil, fmt.Errorf("cannot index %s with number", v.typeName())
	}
	n := len(v.arrVal)
	if idx < 0 {
		idx = n + idx
	}
	if idx < 0 || idx >= n {
		return &Value{kind: kindNull}, nil
	}
	return v.arrVal[idx], nil
}

// iterate returns all elements of an array, all values of a dict, or all keys of a set.
func (v *Value) iterate() ([]*Value, error) {
	switch v.kind {
	case kindArray:
		return v.arrVal, nil
	case kindDict:
		result := make([]*Value, len(v.dictKeys))
		for i, k := range v.dictKeys {
			result[i] = v.dictVals[k]
		}
		return result, nil
	case kindSet:
		result := make([]*Value, len(v.setKeys))
		for i, k := range v.setKeys {
			result[i] = &Value{kind: kindString, strVal: k}
		}
		return result, nil
	default:
		return nil, fmt.Errorf("cannot iterate %s", v.typeName())
	}
}

// keys returns the keys of a dict or set as a sorted or ordered list Value.
func (v *Value) keys(sorted bool) (*Value, error) {
	var ks []string
	switch v.kind {
	case kindDict:
		ks = make([]string, len(v.dictKeys))
		copy(ks, v.dictKeys)
	case kindSet:
		ks = make([]string, len(v.setKeys))
		copy(ks, v.setKeys)
	default:
		return nil, fmt.Errorf("keys requires object or set, got %s", v.typeName())
	}
	if sorted {
		sort.Strings(ks)
	}
	arr := make([]*Value, len(ks))
	for i, k := range ks {
		arr[i] = &Value{kind: kindString, strVal: k}
	}
	return &Value{kind: kindArray, arrVal: arr}, nil
}

// length returns the length of the value.
func (v *Value) length() (int, error) {
	switch v.kind {
	case kindNull:
		return 0, nil
	case kindString:
		return len([]rune(v.strVal)), nil
	case kindArray:
		return len(v.arrVal), nil
	case kindDict:
		return len(v.dictKeys), nil
	case kindSet:
		return len(v.setKeys), nil
	default:
		return 0, fmt.Errorf("cannot get length of %s", v.typeName())
	}
}

// truthy returns the boolean truthiness of the value.
func (v *Value) truthy() bool {
	switch v.kind {
	case kindNull:
		return false
	case kindBool:
		return v.boolVal
	default:
		return true
	}
}

// sortedCopy returns a copy of a dict value with alphabetically sorted keys.
func (v *Value) sortedCopy() *Value {
	if v.kind != kindDict {
		return v
	}
	keys := make([]string, len(v.dictKeys))
	copy(keys, v.dictKeys)
	sort.Strings(keys)
	return &Value{
		kind:     kindDict,
		dictKeys: keys,
		dictVals: v.dictVals,
	}
}
