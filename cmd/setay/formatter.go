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
	// The document IS the top-level dict's braces; emit them, preserving the
	// author's single-line vs multi-line choice.
	f.formatTopLevelDict(doc.Dict)
}

func (f *formatter) formatTopLevelDict(dict *DefSetayDict) {
	multiline := f.dictIsMultiline(dict)

	if len(dict.Entries) == 0 {
		if multiline {
			f.out.WriteString("{\n")
			f.emitBlockInnerSpacing(dict.AnonymousField1.GetAuthority(), dict.AnonymousField4.GetAuthority(), 1)
			f.out.WriteString("}\n")
		} else {
			f.out.WriteString("{}\n")
		}
		return
	}

	if multiline {
		f.out.WriteString("{\n")
		f.formatDictEntriesMultiLine(dict, 1)
		f.out.WriteString("}\n")
	} else {
		f.formatDictSingleLine(dict, 0)
		f.out.WriteByte('\n')
	}
}

// formatDictEntriesMultiLine formats dict entries in multi-line style.
func (f *formatter) formatDictEntriesMultiLine(dict *DefSetayDict, depth int) {
	entries := dict.Entries[0]
	allEntries := f.collectDictEntries(entries)
	allSpacings := f.collectDictSpacings(dict, entries)

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
	// A multi-line dict or list value goes on its own line beneath the key
	// (Allman): "key =\n<indent>{" or "key =\n<indent>[". Everything else stays
	// on the same line: "key = value".
	if f.valueIsMultilineBlock(entry.Value) {
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
// inline: if true, a multi-line dict/list opens its delimiter on the current
// line (for list elements). If false, a multi-line dict/list opens on its own
// line at depth (Allman, for dict-entry values).
func (f *formatter) formatValueInner(val *DefSetayValue, depth int, inline bool) {
	inner := val.AnonymousField1
	switch v := inner.(type) {
	case *DefSetayDict:
		f.formatDict(v, depth, inline)
	case *DefSetayList:
		f.formatList(v, depth, inline)
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

// formatDict formats a dict value, preserving the author's single-line vs
// multi-line layout (Setay does not decide this for the author; see
// docs/setay-aesthetics.ja.md). inline controls where the opening '{' of a
// multi-line dict goes: false → its own line at depth (Allman, dict-entry
// values); true → the current line (list elements).
func (f *formatter) formatDict(dict *DefSetayDict, depth int, inline bool) {
	multiline := f.dictIsMultiline(dict)

	if len(dict.Entries) == 0 {
		if !multiline {
			f.out.WriteString("{}")
			return
		}
		// Preserve an intentionally split-open empty dict (e.g. content is about
		// to be added), keeping any comments inside it.
		f.openBlock(depth, inline, '{')
		f.emitBlockInnerSpacing(dict.AnonymousField1.GetAuthority(), dict.AnonymousField4.GetAuthority(), depth+1)
		f.writeIndent(depth)
		f.out.WriteByte('}')
		return
	}

	if !multiline {
		f.formatDictSingleLine(dict, depth)
		return
	}

	f.openBlock(depth, inline, '{')
	f.formatDictEntriesMultiLine(dict, depth+1)
	f.writeIndent(depth)
	f.out.WriteByte('}')
}

// formatDictSingleLine renders a dict on one line: "{ k = v; k = v }".
func (f *formatter) formatDictSingleLine(dict *DefSetayDict, depth int) {
	f.out.WriteString("{ ")
	for i, entry := range f.collectDictEntries(dict.Entries[0]) {
		if i > 0 {
			f.out.WriteString("; ")
		}
		f.formatDictEntry(entry, depth)
	}
	f.out.WriteString(" }")
}

// formatList formats a list value, preserving the author's single-line vs
// multi-line layout. inline behaves as in formatDict; a multi-line list shares
// the '{' positioning rule — its '[' is placed the same way.
func (f *formatter) formatList(list *DefSetayList, depth int, inline bool) {
	multiline := f.listIsMultiline(list)

	if len(list.Elements) == 0 {
		if !multiline {
			f.out.WriteString("[]")
			return
		}
		f.openBlock(depth, inline, '[')
		f.emitBlockInnerSpacing(list.AnonymousField1.GetAuthority(), list.AnonymousField4.GetAuthority(), depth+1)
		f.writeIndent(depth)
		f.out.WriteByte(']')
		return
	}

	elements := list.Elements[0]
	allValues := f.collectListValues(elements)

	if !multiline {
		f.out.WriteString("[ ")
		for i, val := range allValues {
			if i > 0 {
				f.out.WriteString(", ")
			}
			f.formatValueInner(val, depth, true)
		}
		f.out.WriteString(" ]")
		return
	}

	allSpacings := f.collectListSpacings(list, elements)
	f.openBlock(depth, inline, '[')
	for i, val := range allValues {
		if i < len(allSpacings) {
			f.emitInterEntrySpacing(allSpacings[i], depth+1)
		}
		f.writeIndent(depth + 1)
		f.formatValueInner(val, depth+1, true)
		f.out.WriteByte(',')
		f.out.WriteByte('\n')
	}
	if len(allSpacings) > len(allValues) {
		f.emitInterEntrySpacing(allSpacings[len(allValues)], depth+1)
	}
	f.writeIndent(depth)
	f.out.WriteByte(']')
}

// openBlock writes the opening delimiter of a multi-line block plus its newline.
// inline=false places the delimiter on its own line at depth (Allman); true
// opens it on the current line.
func (f *formatter) openBlock(depth int, inline bool, delim byte) {
	if !inline {
		f.out.WriteByte('\n')
		f.writeIndent(depth)
	}
	f.out.WriteByte(delim)
	f.out.WriteByte('\n')
}

// emitBlockInnerSpacing emits the comments/blank lines that sit between an empty
// block's delimiters, re-indented to depth.
func (f *formatter) emitBlockInnerSpacing(open, close *Authority, depth int) {
	region := f.sourceSlice(int(open.StartedAt)+int(open.Length), int(close.StartedAt))
	f.emitInterEntrySpacing(f.parseSpacingInfo(region), depth)
}

// dictIsMultiline reports whether the author wrote this dict across multiple
// lines (a newline between its braces). Setay preserves this choice rather than
// deciding single vs multi-line by a column budget of its own.
func (f *formatter) dictIsMultiline(dict *DefSetayDict) bool {
	return f.blockHasNewline(dict.AnonymousField1.GetAuthority(), dict.AnonymousField4.GetAuthority())
}

// listIsMultiline reports whether the author wrote this list across multiple lines.
func (f *formatter) listIsMultiline(list *DefSetayList) bool {
	return f.blockHasNewline(list.AnonymousField1.GetAuthority(), list.AnonymousField4.GetAuthority())
}

// blockHasNewline reports whether the source between a block's opening and
// closing delimiter contains a newline. (A single-line line comment inside a
// block necessarily carries a newline before the closing delimiter, so this
// also captures "has comments".)
func (f *formatter) blockHasNewline(open, close *Authority) bool {
	region := f.sourceSlice(int(open.StartedAt)+int(open.Length), int(close.StartedAt))
	return strings.ContainsRune(region, '\n')
}

// valueIsMultilineBlock reports whether a value is a dict or list the author
// wrote across multiple lines (so it should open Allman-style under its key).
func (f *formatter) valueIsMultilineBlock(val *DefSetayValue) bool {
	switch v := val.AnonymousField1.(type) {
	case *DefSetayDict:
		return f.dictIsMultiline(v)
	case *DefSetayList:
		return f.listIsMultiline(v)
	}
	return false
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
func (f *formatter) collectDictSpacings(dict *DefSetayDict, entries *DefSetayDictEntries) []spacingInfo {
	var spacings []spacingInfo

	firstAuth := entries.First.GetAuthority()

	// Before first entry: the leading region (comments/blank lines between '{'
	// and the first entry) lives in the DICT node, not the entries node —
	// DefSetayDictEntries starts AT the first entry, so slicing from its start
	// is always empty and would drop every leading comment. Slice from just
	// after the opening '{' instead.
	braceOpen := dict.AnonymousField1.GetAuthority()
	braceOpenEnd := int(braceOpen.StartedAt) + int(braceOpen.Length)
	beforeFirst := f.sourceSlice(braceOpenEnd, int(firstAuth.StartedAt))
	spacings = append(spacings, f.parseSpacingInfo(beforeFirst))

	// Between entries: from previous entry's value end (+';') to next entry's key start
	prevEnd := int(firstAuth.StartedAt) + int(firstAuth.Length)
	for _, sep := range entries.Rest {
		entryAuth := sep.Entry.GetAuthority()
		between := f.sourceSlice(prevEnd, int(entryAuth.StartedAt))
		spacings = append(spacings, f.parseSpacingInfo(between))
		prevEnd = int(entryAuth.StartedAt) + int(entryAuth.Length)
	}

	// After last entry: up to the closing '}'. Using the brace start (rather than
	// the entries node end) captures trailing comments that sit in the dict's
	// trailing spacing region.
	braceClose := dict.AnonymousField4.GetAuthority()
	afterLast := f.sourceSlice(prevEnd, int(braceClose.StartedAt))
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
// Mirrors collectDictSpacings: the leading region (between '[' and the first
// element) and trailing region (before ']') live in the LIST node, not the
// elements node, so they are sliced from the brackets.
func (f *formatter) collectListSpacings(list *DefSetayList, elements *DefSetayListElements) []spacingInfo {
	var spacings []spacingInfo

	firstAuth := elements.First.GetAuthority()

	bracketOpen := list.AnonymousField1.GetAuthority()
	bracketOpenEnd := int(bracketOpen.StartedAt) + int(bracketOpen.Length)
	beforeFirst := f.sourceSlice(bracketOpenEnd, int(firstAuth.StartedAt))
	spacings = append(spacings, f.parseSpacingInfo(beforeFirst))

	prevEnd := int(firstAuth.StartedAt) + int(firstAuth.Length)
	for _, sep := range elements.Rest {
		valAuth := sep.Value.GetAuthority()
		between := f.sourceSlice(prevEnd, int(valAuth.StartedAt))
		spacings = append(spacings, f.parseSpacingInfo(between))
		prevEnd = int(valAuth.StartedAt) + int(valAuth.Length)
	}

	bracketClose := list.AnonymousField4.GetAuthority()
	afterLast := f.sourceSlice(prevEnd, int(bracketClose.StartedAt))
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

		if ch == '#' {
			if i+1 < len(runes) && runes[i+1] == '{' {
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
			// Any other '#' in a spacing region starts a single-line comment:
			// "# ", "#\t", "##", "#!", or an empty "#" (a '#' immediately followed
			// by a newline or end-of-input). Because the source already parsed,
			// a '#' inside a spacing region is always a valid comment lead — never
			// stray text — so it is safe to consume to end of line. (The earlier
			// version only recognized ' ', '\t', '#', '!' after the '#', which
			// silently dropped empty "#" comment lines.)
			lineEnd := i
			for lineEnd < len(runes) && runes[lineEnd] != '\n' {
				lineEnd++
			}
			commentText := strings.TrimRight(string(runes[i:lineEnd]), " \t")
			info.items = append(info.items, spacingItem{kind: spacingComment, text: commentText})
			i = lineEnd
			continue
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
