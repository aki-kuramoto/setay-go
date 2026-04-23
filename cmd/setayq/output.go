package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
)

// outputOptions controls how values are printed.
type outputOptions struct {
	compact     bool
	rawOutput   bool
	sortKeys    bool
	toJSON      bool
	setAsObject bool // JSON output: render kindSet as {"key":true} instead of ["key"]
	indent      string
	color       bool // emit ANSI color escape sequences
}

// ---- ANSI color helpers ----

const (
	ansiReset  = "\033[0m"
	ansiKey    = "\033[36m" // cyan  — dict/set keys
	ansiStr    = "\033[32m" // green — string values
	ansiNum    = "\033[33m" // yellow — numbers
	ansiKw     = "\033[35m" // magenta — true/false/null/timestamps
)

func colorize(color, s string, useColor bool) string {
	if !useColor {
		return s
	}
	return color + s + ansiReset
}

// ---- setay output ----

func printSetay(v *Value, opts outputOptions) error {
	if opts.sortKeys && v.kind == kindDict {
		v = v.sortedCopy()
	}
	if opts.compact {
		s := valueToSetayCompact(v, opts.rawOutput, opts.color)
		fmt.Println(s)
	} else {
		s := valueToSetayPretty(v, 0, opts.indent, opts.rawOutput, opts.color)
		fmt.Print(s)
		if !strings.HasSuffix(s, "\n") {
			fmt.Println()
		}
	}
	return nil
}

func valueToSetayString(v *Value) string {
	return valueToSetayCompact(v, false, false)
}

func valueToSetayCompact(v *Value, raw bool, color bool) string {
	switch v.kind {
	case kindNull:
		return colorize(ansiKw, "null", color)
	case kindBool:
		if v.boolVal {
			return colorize(ansiKw, "true", color)
		}
		return colorize(ansiKw, "false", color)
	case kindNumber:
		return colorize(ansiNum, formatNumber(v.numFloat), color)
	case kindString:
		if raw {
			return colorize(ansiStr, v.strVal, color)
		}
		return colorize(ansiStr, escapeSetayString(v.strVal), color)
	case kindArray:
		var parts []string
		for _, item := range v.arrVal {
			parts = append(parts, valueToSetayCompact(item, false, color))
		}
		return "[ " + strings.Join(parts, ", ") + " ]"
	case kindDict:
		var parts []string
		for _, k := range v.dictKeys {
			parts = append(parts, colorize(ansiKey, formatKey(k), color)+" = "+valueToSetayCompact(v.dictVals[k], false, color))
		}
		return "{ " + strings.Join(parts, "; ") + " }"
	case kindSet:
		var parts []string
		for _, k := range v.setKeys {
			parts = append(parts, colorize(ansiKey, formatKey(k), color)+"=")
		}
		return "{ " + strings.Join(parts, "; ") + " }"
	case kindTimestamp:
		return colorize(ansiKw, `UtcTs("`+v.strVal+`")`, color)
	case kindVarRef:
		return "${" + v.varName + "}"
	}
	return colorize(ansiKw, "null", color)
}

func valueToSetayPretty(v *Value, depth int, indent string, raw bool, color bool) string {
	switch v.kind {
	case kindNull:
		return colorize(ansiKw, "null", color) + "\n"
	case kindBool:
		if v.boolVal {
			return colorize(ansiKw, "true", color) + "\n"
		}
		return colorize(ansiKw, "false", color) + "\n"
	case kindNumber:
		return colorize(ansiNum, formatNumber(v.numFloat), color) + "\n"
	case kindString:
		if raw {
			return colorize(ansiStr, v.strVal, color) + "\n"
		}
		return colorize(ansiStr, escapeSetayString(v.strVal), color) + "\n"
	case kindArray:
		if len(v.arrVal) == 0 {
			return "[]\n"
		}
		var sb strings.Builder
		sb.WriteString("[\n")
		for _, item := range v.arrVal {
			sb.WriteString(strings.Repeat(indent, depth+1))
			s := valueToSetayPretty(item, depth+1, indent, false, color)
			sb.WriteString(strings.TrimRight(s, "\n"))
			sb.WriteString(",\n")
		}
		sb.WriteString(strings.Repeat(indent, depth))
		sb.WriteString("]\n")
		return sb.String()
	case kindDict:
		if len(v.dictKeys) == 0 {
			return "{}\n"
		}
		var sb strings.Builder
		sb.WriteString("{\n")
		ind := strings.Repeat(indent, depth+1)
		for i, k := range v.dictKeys {
			sb.WriteString(ind)
			sb.WriteString(colorize(ansiKey, formatKey(k), color))
			sb.WriteString(" = ")
			val := v.dictVals[k]
			s := valueToSetayPretty(val, depth+1, indent, false, color)
			sb.WriteString(strings.TrimRight(s, "\n"))
			if i < len(v.dictKeys)-1 {
				sb.WriteString(";")
			}
			sb.WriteString("\n")
		}
		sb.WriteString(strings.Repeat(indent, depth))
		sb.WriteString("}\n")
		return sb.String()
	case kindSet:
		if len(v.setKeys) == 0 {
			return "{=;}\n"
		}
		var sb strings.Builder
		sb.WriteString("{\n")
		ind := strings.Repeat(indent, depth+1)
		for _, k := range v.setKeys {
			sb.WriteString(ind)
			sb.WriteString(colorize(ansiKey, formatKey(k), color))
			sb.WriteString("=;\n")
		}
		sb.WriteString(strings.Repeat(indent, depth))
		sb.WriteString("}\n")
		return sb.String()
	case kindTimestamp:
		return colorize(ansiKw, `UtcTs("`+v.strVal+`")`, color) + "\n"
	case kindVarRef:
		return "${" + v.varName + "}\n"
	}
	return colorize(ansiKw, "null", color) + "\n"
}

func formatKey(k string) string {
	if isBareKey(k) {
		return k
	}
	return escapeSetayString(k)
}

func isBareKey(s string) bool {
	if len(s) == 0 {
		return false
	}
	if !(s[0] >= 'a' && s[0] <= 'z' || s[0] >= 'A' && s[0] <= 'Z' || s[0] == '_') {
		return false
	}
	for _, c := range s[1:] {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
			return false
		}
	}
	return true
}

func escapeSetayString(s string) string {
	var sb strings.Builder
	sb.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			sb.WriteString(`\"`)
		case '\\':
			sb.WriteString(`\\`)
		case '\n':
			sb.WriteString(`\n`)
		case '\r':
			sb.WriteString(`\r`)
		case '\t':
			sb.WriteString(`\t`)
		default:
			if r < 0x20 {
				sb.WriteString(fmt.Sprintf(`\x%02X`, r))
			} else {
				sb.WriteRune(r)
			}
		}
	}
	sb.WriteByte('"')
	return sb.String()
}

func formatNumber(f float64) string {
	if math.IsInf(f, 1) {
		return "1e308"
	}
	if math.IsInf(f, -1) {
		return "-1e308"
	}
	if math.IsNaN(f) {
		return "null"
	}
	if f == math.Trunc(f) && math.Abs(f) < 1e15 {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}

// ---- JSON output ----

func valueToJSON(v *Value, opts outputOptions) ([]byte, error) {
	b, err := valueToJSONBytes(v, opts)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func valueToJSONBytes(v *Value, opts outputOptions) ([]byte, error) {
	raw := valueToJSONInterface(v, opts)
	if opts.compact {
		return json.Marshal(raw)
	}
	indent := opts.indent
	if indent == "" {
		indent = "  "
	}
	return json.MarshalIndent(raw, "", indent)
}

func valueToJSONInterface(v *Value, opts outputOptions) interface{} {
	switch v.kind {
	case kindNull:
		return nil
	case kindBool:
		return v.boolVal
	case kindNumber:
		if v.numFloat == math.Trunc(v.numFloat) && math.Abs(v.numFloat) < 1e15 {
			return int64(v.numFloat)
		}
		return v.numFloat
	case kindString:
		return v.strVal
	case kindArray:
		arr := make([]interface{}, len(v.arrVal))
		for i, item := range v.arrVal {
			arr[i] = valueToJSONInterface(item, opts)
		}
		return arr
	case kindDict:
		keys := v.dictKeys
		if opts.sortKeys {
			keys = make([]string, len(v.dictKeys))
			copy(keys, v.dictKeys)
			sort.Strings(keys)
		}
		m := make(map[string]interface{}, len(keys))
		for _, k := range keys {
			m[k] = valueToJSONInterface(v.dictVals[k], opts)
		}
		return m
	case kindSet:
		if opts.setAsObject {
			// Render set as {"key1": true, "key2": true}
			m := make(map[string]interface{}, len(v.setKeys))
			for _, k := range v.setKeys {
				m[k] = true
			}
			return m
		}
		// default: output as array of keys
		arr := make([]interface{}, len(v.setKeys))
		for i, k := range v.setKeys {
			arr[i] = k
		}
		return arr
	case kindTimestamp:
		return v.tsVal.UTC().Format("2006-01-02T15:04:05Z")
	case kindVarRef:
		return "${" + v.varName + "}"
	}
	return nil
}

// jsonToValue converts a JSON string to a *Value.
func jsonToValue(s string) (*Value, error) {
	var raw interface{}
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		return nil, fmt.Errorf("invalid JSON: %v", err)
	}
	return interfaceToValue(raw), nil
}

func interfaceToValue(raw interface{}) *Value {
	if raw == nil {
		return &Value{kind: kindNull}
	}
	switch v := raw.(type) {
	case bool:
		return &Value{kind: kindBool, boolVal: v}
	case float64:
		return &Value{kind: kindNumber, numFloat: v, numText: strconv.FormatFloat(v, 'g', -1, 64)}
	case string:
		return &Value{kind: kindString, strVal: v}
	case []interface{}:
		arr := make([]*Value, len(v))
		for i, item := range v {
			arr[i] = interfaceToValue(item)
		}
		return &Value{kind: kindArray, arrVal: arr}
	case map[string]interface{}:
		result := &Value{kind: kindDict, dictVals: make(map[string]*Value)}
		for k, val := range v {
			result.dictKeys = append(result.dictKeys, k)
			result.dictVals[k] = interfaceToValue(val)
		}
		sort.Strings(result.dictKeys) // stable order
		return result
	}
	return &Value{kind: kindNull}
}

// Ensure os is imported for env builtin
var _ = os.Environ
