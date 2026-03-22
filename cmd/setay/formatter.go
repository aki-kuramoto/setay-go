package main

import (
	"fmt"
	"strings"
)

// formatter formats a parsed setay document into the standard style.
type formatter struct {
	source []rune
	out    strings.Builder
}

// Format formats the parsed document and returns the formatted string.
func Format(source string, doc *DefSetayDocument) string {
	f := &formatter{source: []rune(source)}
	f.formatDocument(doc)
	return f.out.String()
}

func (f *formatter) formatDocument(doc *DefSetayDocument) {
	// Emit leading comments/spacing (before the top-level dict's '{')
	if doc.Dict != nil {
		f.emitLeadingSpacing(doc.Dict)
	}
	// Top-level dict: always multi-line, no surrounding braces at top level — wait,
	// setay top-level IS the braces. So we emit them.
	f.formatTopLevelDict(doc.Dict)
}

func (f *formatter) formatTopLevelDict(dict *DefSetayDict) {
	f.out.WriteString("{\n")
	if len(dict.Entries) > 0 {
		entries := dict.Entries[0]
		f.formatDictEntriesMultiLine(entries, 1)
	}
	f.out.WriteString("}\n")
}

// formatDictEntriesMultiLine formats dict entries in multi-line style.
func (f *formatter) formatDictEntriesMultiLine(entries *DefSetayDictEntries, depth int) {
	allEntries := f.collectDictEntries(entries)
	allSpacings := f.collectDictSpacings(entries)

	for i, entry := range allEntries {
		// Emit blank lines / comments from spacing before this entry
		if i < len(allSpacings) {
			f.emitInterEntrySpacing(allSpacings[i], depth)
		}
		f.writeIndent(depth)
		f.formatDictEntry(entry, depth)
		// Multi-line: always emit semicolon
		f.out.WriteByte(';')
		f.out.WriteByte('\n')
	}

	// Spacing after the last entry (before closing brace)
	if len(allSpacings) > len(allEntries) {
		f.emitInterEntrySpacing(allSpacings[len(allEntries)], depth)
	}
}

// formatDictEntry formats a single key = value entry.
func (f *formatter) formatDictEntry(entry *DefSetayDictEntry, depth int) {
	f.formatDictKey(entry.Key)
	// For non-empty dict values, use " =\n" (no trailing space before newline)
	if dict, ok := entry.Value.AnonymousField1.(*DefSetayDict); ok && len(dict.Entries) > 0 {
		f.out.WriteString(" =")
	} else {
		f.out.WriteString(" = ")
	}
	f.formatValue(entry.Value, depth)
}

// formatDictKey outputs the key, preserving bare keys and quoted strings.
func (f *formatter) formatDictKey(key *DefSetayDictKey) {
	f.out.WriteString(f.textOf(key.GetAuthority()))
}

// formatValue dispatches to the appropriate formatting function based on the value type.
func (f *formatter) formatValue(val *DefSetayValue, depth int) {
	f.formatValueInner(val, depth, false)
}

// formatValueInner dispatches to the appropriate formatting function.
// inlineDict: if true, dict opens with '{' on the current line (for list elements).
func (f *formatter) formatValueInner(val *DefSetayValue, depth int, inlineDict bool) {
	inner := val.AnonymousField1
	switch v := inner.(type) {
	case *DefSetayDict:
		f.formatDict(v, depth, inlineDict)
	case *DefSetayList:
		f.formatList(v, depth)
	case *DefSetayUtcTs:
		f.out.WriteString(f.textOf(v.GetAuthority()))
	case *DefSetayTrue:
		f.out.WriteString("true")
	case *DefSetayFalse:
		f.out.WriteString("false")
	case *DefSetayNull:
		f.out.WriteString("null")
	case *DefSetayNumber:
		f.out.WriteString(f.textOf(v.GetAuthority()))
	case *DefSetayString:
		f.out.WriteString(f.textOf(v.GetAuthority()))
	default:
		// Fallback: original text
		f.out.WriteString(f.textOf(val.GetAuthority()))
	}
}

// formatDict formats a dict value. Empty dict → `{}`. Otherwise → multi-line.
func (f *formatter) formatDict(dict *DefSetayDict, depth int, inline bool) {
	if len(dict.Entries) == 0 {
		f.out.WriteString("{}")
		return
	}

	if inline {
		// Inline mode (e.g., list element): open '{' on current line
		f.out.WriteString("{\n")
	} else {
		// Dict entry value: "key =\n<indent>{"
		f.out.WriteByte('\n')
		f.writeIndent(depth)
		f.out.WriteString("{\n")
	}

	entries := dict.Entries[0]
	f.formatDictEntriesMultiLine(entries, depth+1)

	f.writeIndent(depth)
	f.out.WriteByte('}')
}

// formatList formats a list value.
func (f *formatter) formatList(list *DefSetayList, depth int) {
	if len(list.Elements) == 0 {
		f.out.WriteString("[]")
		return
	}

	elements := list.Elements[0]
	allValues := f.collectListValues(elements)

	// Decide single-line vs multi-line.
	// Single-line if all values are simple (not Dict/List) and total length is short enough.
	if f.canFormatListSingleLine(allValues, depth) {
		f.out.WriteString("[ ")
		for i, val := range allValues {
			if i > 0 {
				f.out.WriteString(", ")
			}
			f.formatValue(val, depth)
		}
		f.out.WriteString(" ]")
	} else {
		f.out.WriteString("[\n")
		allSpacings := f.collectListSpacings(elements)
		for i, val := range allValues {
			if i < len(allSpacings) {
				f.emitInterEntrySpacing(allSpacings[i], depth+1)
			}
			f.writeIndent(depth + 1)
			// Use inline mode for dicts inside lists
			f.formatValueInner(val, depth+1, true)
			// Multi-line: always emit comma
			f.out.WriteByte(',')
			f.out.WriteByte('\n')
		}
		if len(allSpacings) > len(allValues) {
			f.emitInterEntrySpacing(allSpacings[len(allValues)], depth+1)
		}
		f.writeIndent(depth)
		f.out.WriteByte(']')
	}
}

// canFormatListSingleLine checks if a list can be formatted on a single line.
func (f *formatter) canFormatListSingleLine(values []*DefSetayValue, depth int) bool {
	totalLen := 4 // "[ " + " ]"
	for i, val := range values {
		if i > 0 {
			totalLen += 2 // ", "
		}
		inner := val.AnonymousField1
		switch inner.(type) {
		case *DefSetayDict, *DefSetayList:
			return false // Nested structures → always multi-line
		}
		totalLen += int(val.GetAuthority().Length)
	}
	// Include indent in line length estimate
	maxLine := 80
	return totalLen+(depth*4) <= maxLine
}

// collectDictEntries collects all dict entries from the entries node.
func (f *formatter) collectDictEntries(entries *DefSetayDictEntries) []*DefSetayDictEntry {
	result := []*DefSetayDictEntry{entries.First}
	for _, sep := range entries.Rest {
		result = append(result, sep.Entry)
	}
	return result
}

// collectDictSpacings collects spacing regions between dict entries.
// Returns n+1 spacing texts for n entries:
// [before_first, between_1_2, between_2_3, ..., after_last]
func (f *formatter) collectDictSpacings(entries *DefSetayDictEntries) []spacingInfo {
	var spacings []spacingInfo

	// Before first entry: spacing from dict's '{' to first entry's key start
	firstAuth := entries.First.GetAuthority()
	entriesAuth := entries.GetAuthority()
	beforeFirst := f.sourceSlice(int(entriesAuth.StartedAt), int(firstAuth.StartedAt))
	spacings = append(spacings, f.parseSpacingInfo(beforeFirst))

	// Between entries: from previous entry's value end (+';') to next entry's key start
	prevEnd := int(firstAuth.StartedAt) + int(firstAuth.Length)
	for _, sep := range entries.Rest {
		entryAuth := sep.Entry.GetAuthority()
		between := f.sourceSlice(prevEnd, int(entryAuth.StartedAt))
		spacings = append(spacings, f.parseSpacingInfo(between))
		prevEnd = int(entryAuth.StartedAt) + int(entryAuth.Length)
	}

	// After last entry
	afterLast := f.sourceSlice(prevEnd, int(entriesAuth.StartedAt)+int(entriesAuth.Length))
	spacings = append(spacings, f.parseSpacingInfo(afterLast))

	return spacings
}

// collectListValues collects all values from list elements.
func (f *formatter) collectListValues(elements *DefSetayListElements) []*DefSetayValue {
	result := []*DefSetayValue{elements.First}
	for _, sep := range elements.Rest {
		result = append(result, sep.Value)
	}
	return result
}

// collectListSpacings collects spacing info between list elements.
func (f *formatter) collectListSpacings(elements *DefSetayListElements) []spacingInfo {
	var spacings []spacingInfo

	firstAuth := elements.First.GetAuthority()
	elemAuth := elements.GetAuthority()
	beforeFirst := f.sourceSlice(int(elemAuth.StartedAt), int(firstAuth.StartedAt))
	spacings = append(spacings, f.parseSpacingInfo(beforeFirst))

	prevEnd := int(firstAuth.StartedAt) + int(firstAuth.Length)
	for _, sep := range elements.Rest {
		valAuth := sep.Value.GetAuthority()
		between := f.sourceSlice(prevEnd, int(valAuth.StartedAt))
		spacings = append(spacings, f.parseSpacingInfo(between))
		prevEnd = int(valAuth.StartedAt) + int(valAuth.Length)
	}

	afterLast := f.sourceSlice(prevEnd, int(elemAuth.StartedAt)+int(elemAuth.Length))
	spacings = append(spacings, f.parseSpacingInfo(afterLast))

	return spacings
}

// spacingInfo holds information about a spacing region.
type spacingInfo struct {
	// items preserves the order of blank lines and comments as they appeared.
	items []spacingItem
}

type spacingItemKind int

const (
	spacingBlankLine spacingItemKind = iota
	spacingComment
)

type spacingItem struct {
	kind spacingItemKind
	text string // For comments: the full comment text (including #)
}

// parseSpacingInfo analyzes a spacing region to detect blank lines and comments.
func (f *formatter) parseSpacingInfo(text string) spacingInfo {
	info := spacingInfo{}
	cleaned := strings.ReplaceAll(text, "\r", "")

	i := 0
	runes := []rune(cleaned)
	sawNewline := false

	for i < len(runes) {
		ch := runes[i]

		if ch == '\n' {
			if sawNewline {
				// Consecutive newline = blank line
				info.items = append(info.items, spacingItem{kind: spacingBlankLine})
			}
			sawNewline = true
			i++
			continue
		}

		if ch == ' ' || ch == '\t' {
			i++
			continue
		}

		sawNewline = false

		if ch == '#' && i+1 < len(runes) {
			next := runes[i+1]
			if next == '{' {
				// Multi-line comment: find "}#" in rune slice
				commentEnd := -1
				for k := i + 2; k+1 < len(runes); k++ {
					if runes[k] == '}' && runes[k+1] == '#' {
						commentEnd = k + 2 // past "}#"
						break
					}
				}
				if commentEnd >= 0 {
					commentText := string(runes[i:commentEnd])
					info.items = append(info.items, spacingItem{kind: spacingComment, text: commentText})
					i = commentEnd
					continue
				}
				// Unclosed — treat rest as comment
				info.items = append(info.items, spacingItem{kind: spacingComment, text: string(runes[i:])})
				break
			}
			if next == ' ' || next == '\t' || next == '#' || next == '!' {
				// Single-line comment: consume until newline
				lineEnd := i
				for lineEnd < len(runes) && runes[lineEnd] != '\n' {
					lineEnd++
				}
				commentText := strings.TrimRight(string(runes[i:lineEnd]), " \t")
				info.items = append(info.items, spacingItem{kind: spacingComment, text: commentText})
				i = lineEnd
				continue
			}
		}

		// Other characters: skip (shouldn't normally appear in spacing)
		i++
	}

	return info
}

// emitInterEntrySpacing outputs blank lines and comments between entries.
func (f *formatter) emitInterEntrySpacing(info spacingInfo, depth int) {
	for _, item := range info.items {
		switch item.kind {
		case spacingBlankLine:
			f.writeIndent(depth)
			f.out.WriteByte('\n')
		case spacingComment:
			if strings.Contains(item.text, "\n") {
				// Multi-line comment: strip existing indent and re-indent
				lines := strings.Split(item.text, "\n")
				for j, line := range lines {
					f.writeIndent(depth)
					trimmed := strings.TrimLeft(line, " \t")
					f.out.WriteString(trimmed)
					if j < len(lines)-1 {
						f.out.WriteByte('\n')
					}
				}
				f.out.WriteByte('\n')
			} else {
				f.writeIndent(depth)
				f.out.WriteString(item.text)
				f.out.WriteByte('\n')
			}
		}
	}
}

// emitLeadingSpacing emits comments before the top-level dict.
func (f *formatter) emitLeadingSpacing(dict *DefSetayDict) {
	dictStart := int(dict.GetAuthority().StartedAt)
	if dictStart > 0 {
		leading := f.sourceSlice(0, dictStart)
		info := f.parseSpacingInfo(leading)
		for _, item := range info.items {
			if item.kind == spacingComment {
				f.out.WriteString(item.text)
				f.out.WriteByte('\n')
			}
		}
	}
}

// writeIndent writes tab indentation.
func (f *formatter) writeIndent(depth int) {
	for i := 0; i < depth; i++ {
		f.out.WriteByte('\t')
	}
}

// textOf extracts the original text of a node from the source.
func (f *formatter) textOf(auth *Authority) string {
	start := int(auth.StartedAt)
	end := start + int(auth.Length)
	if end > len(f.source) {
		end = len(f.source)
	}
	return string(f.source[start:end])
}

// sourceSlice extracts a substring from the source runes.
func (f *formatter) sourceSlice(start, end int) string {
	if start < 0 {
		start = 0
	}
	if end > len(f.source) {
		end = len(f.source)
	}
	if start >= end {
		return ""
	}
	return string(f.source[start:end])
}

// Ensure DefSetayUtcTs and DefSetayNumber are handled
var _ = (*DefSetayUtcTs)(nil)
var _ = (*DefSetayNumber)(nil)
var _ = fmt.Sprintf
