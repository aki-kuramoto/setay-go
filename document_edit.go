package setay

import "fmt"

// Root returns a Node for the document's top-level dict, so that entries can be
// appended or prepended at the top level. It is a container handle only (it has
// no value, key, or position of its own).
func (d *Document) Root() *Node {
	return &Node{doc: d, rootDict: d.root.Dict}
}

// --- Rename ------------------------------------------------------------------

// Rename replaces a dict entry's key with raw setay key text (a bare key or a
// quoted string). n must be a dict entry (obtained by navigating into a dict).
func (cs *ChangeSet) Rename(n *Node, keyText string) error {
	if err := cs.checkNode(n); err != nil {
		return err
	}
	if n.entry == nil {
		return fmt.Errorf("setay: Rename requires a dict entry")
	}
	ks, ke := authSpan(n.entry.Key.GetAuthority())
	return cs.addEdit(ks, ke-ks, keyText)
}

// --- Delete ------------------------------------------------------------------

// Delete removes a dict entry or a list element, together with its separator and
// the whitespace of its line, so no dangling separator or blank line is left.
func (cs *ChangeSet) Delete(n *Node) error {
	if err := cs.checkNode(n); err != nil {
		return err
	}
	switch {
	case n.entry != nil:
		ks, _ := authSpan(n.entry.Key.GetAuthority())
		_, ve := authSpan(n.entry.Value.GetAuthority())
		start, length := cs.doc.lineDeleteSpan(ks, ve, ';')
		return cs.addEdit(start, length, "")
	case n.parentList != nil:
		vs, ve := authSpan(n.value.GetAuthority())
		start, length := cs.doc.lineDeleteSpan(vs, ve, ',')
		return cs.addEdit(start, length, "")
	default:
		return fmt.Errorf("setay: Delete requires a dict entry or list element")
	}
}

// --- Insertion: strict (raw) -------------------------------------------------

// AppendRaw inserts text verbatim just before the container's closing '}' or
// ']'. The container is a Node whose value is a dict or list (or Document.Root).
// The caller supplies the exact text (indentation, separator, newline included).
func (cs *ChangeSet) AppendRaw(container *Node, text string) error {
	if err := cs.checkNode(container); err != nil {
		return err
	}
	_, closeStart, ok := containerDelims(container)
	if !ok {
		return fmt.Errorf("setay: AppendRaw requires a dict or list container")
	}
	return cs.addEdit(closeStart, 0, text)
}

// PrependRaw inserts text verbatim just after the container's opening '{' or '['.
func (cs *ChangeSet) PrependRaw(container *Node, text string) error {
	if err := cs.checkNode(container); err != nil {
		return err
	}
	openEnd, _, ok := containerDelims(container)
	if !ok {
		return fmt.Errorf("setay: PrependRaw requires a dict or list container")
	}
	return cs.addEdit(openEnd, 0, text)
}

// --- Insertion: easy (auto-formatted) ----------------------------------------

// AppendEntry adds a "key = value" entry at the end of a dict, mirroring the
// indentation of the existing last entry (raw key and value text). The value is
// placed inline, so use AppendRaw for a multi-line value or an empty dict.
func (cs *ChangeSet) AppendEntry(dict *Node, keyText, valueText string) error {
	d, ok := containerDict(dict)
	if err := cs.checkNode(dict); err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("setay: AppendEntry requires a dict")
	}
	all := dictEntries(d)
	if len(all) == 0 {
		return fmt.Errorf("setay: AppendEntry into an empty dict is not supported; use AppendRaw")
	}
	last := all[len(all)-1]
	indent := cs.doc.leadingIndent(int(last.Key.GetAuthority().StartedAt))
	_, ve := authSpan(last.Value.GetAuthority())
	if !dictHasTrailingSep(d) {
		if err := cs.addEdit(ve, 0, ";"); err != nil {
			return err
		}
	}
	pos := cs.doc.afterItemLine(ve, ';')
	text := indent + keyText + " = " + valueText + ";" + cs.doc.newline()
	return cs.addEdit(pos, 0, text)
}

// PrependEntry adds a "key = value" entry at the start of a dict, mirroring the
// indentation of the existing first entry.
func (cs *ChangeSet) PrependEntry(dict *Node, keyText, valueText string) error {
	d, ok := containerDict(dict)
	if err := cs.checkNode(dict); err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("setay: PrependEntry requires a dict")
	}
	all := dictEntries(d)
	if len(all) == 0 {
		return fmt.Errorf("setay: PrependEntry into an empty dict is not supported; use PrependRaw")
	}
	indent := cs.doc.leadingIndent(int(all[0].Key.GetAuthority().StartedAt))
	openEnd, _, _ := containerDelims(dict)
	text := cs.doc.newline() + indent + keyText + " = " + valueText + ";"
	return cs.addEdit(openEnd, 0, text)
}

// AppendElem adds a value at the end of a list, mirroring the indentation of the
// existing last element. Use AppendRaw for an empty list or a multi-line value.
func (cs *ChangeSet) AppendElem(list *Node, valueText string) error {
	l, ok := containerList(list)
	if err := cs.checkNode(list); err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("setay: AppendElem requires a list")
	}
	all := listValues(l)
	if len(all) == 0 {
		return fmt.Errorf("setay: AppendElem into an empty list is not supported; use AppendRaw")
	}
	last := all[len(all)-1]
	indent := cs.doc.leadingIndent(int(last.GetAuthority().StartedAt))
	_, ve := authSpan(last.GetAuthority())
	if !listHasTrailingComma(l) {
		if err := cs.addEdit(ve, 0, ","); err != nil {
			return err
		}
	}
	pos := cs.doc.afterItemLine(ve, ',')
	text := indent + valueText + "," + cs.doc.newline()
	return cs.addEdit(pos, 0, text)
}

// PrependElem adds a value at the start of a list, mirroring the first element.
func (cs *ChangeSet) PrependElem(list *Node, valueText string) error {
	l, ok := containerList(list)
	if err := cs.checkNode(list); err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("setay: PrependElem requires a list")
	}
	all := listValues(l)
	if len(all) == 0 {
		return fmt.Errorf("setay: PrependElem into an empty list is not supported; use PrependRaw")
	}
	indent := cs.doc.leadingIndent(int(all[0].GetAuthority().StartedAt))
	openEnd, _, _ := containerDelims(list)
	text := cs.doc.newline() + indent + valueText + ","
	return cs.addEdit(openEnd, 0, text)
}

// --- helpers -----------------------------------------------------------------

func authSpan(a *Authority) (start, end int) {
	return int(a.StartedAt), int(a.StartedAt) + int(a.Length)
}

// containerDelims returns the rune offset just after the opening delimiter and
// just before the closing delimiter of a dict/list container (or Root).
func containerDelims(n *Node) (openEnd, closeStart int, ok bool) {
	if n.rootDict != nil {
		return delimSpans(n.rootDict.AnonymousField1.GetAuthority(), n.rootDict.AnonymousField4.GetAuthority())
	}
	if d, o := dictOf(n.value); o {
		return delimSpans(d.AnonymousField1.GetAuthority(), d.AnonymousField4.GetAuthority())
	}
	if l, o := listOf(n.value); o {
		return delimSpans(l.AnonymousField1.GetAuthority(), l.AnonymousField4.GetAuthority())
	}
	return 0, 0, false
}

func delimSpans(open, close *Authority) (openEnd, closeStart int, ok bool) {
	return int(open.StartedAt) + int(open.Length), int(close.StartedAt), true
}

func containerDict(n *Node) (*DefSetayDict, bool) {
	if n.rootDict != nil {
		return n.rootDict, true
	}
	return dictOf(n.value)
}

func containerList(n *Node) (*DefSetayList, bool) {
	if n.rootDict != nil {
		return nil, false
	}
	return listOf(n.value)
}

func dictHasTrailingSep(d *DefSetayDict) bool {
	if len(d.Entries) == 0 {
		return false
	}
	return len(d.Entries[0].TrailingSemi) > 0
}

func listHasTrailingComma(l *DefSetayList) bool {
	if len(l.Elements) == 0 {
		return false
	}
	return len(l.Elements[0].TrailingComma) > 0
}

// leadingIndent returns the run of spaces/tabs immediately before pos (the
// indentation of pos's line).
func (d *Document) leadingIndent(pos int) string {
	i := pos
	for i > 0 && (d.source[i-1] == ' ' || d.source[i-1] == '\t') {
		i--
	}
	return string(d.source[i:pos])
}

// newline returns the newline sequence the document uses ("\r\n" if it contains
// one, else "\n").
func (d *Document) newline() string {
	for i := 0; i+1 < len(d.source); i++ {
		if d.source[i] == '\r' && d.source[i+1] == '\n' {
			return "\r\n"
		}
		if d.source[i] == '\n' {
			return "\n"
		}
	}
	return "\n"
}

// afterItemLine returns the offset just past an item's line: from itemEnd it
// skips trailing spaces, one optional separator, more spaces, and one optional
// newline.
func (d *Document) afterItemLine(itemEnd int, sep rune) int {
	s := d.source
	le := itemEnd
	for le < len(s) && (s[le] == ' ' || s[le] == '\t') {
		le++
	}
	if le < len(s) && s[le] == sep {
		le++
	}
	for le < len(s) && (s[le] == ' ' || s[le] == '\t') {
		le++
	}
	if le < len(s) && s[le] == '\r' {
		le++
	}
	if le < len(s) && s[le] == '\n' {
		le++
	}
	return le
}

// lineStart returns the offset of the leading indentation of pos's line (just
// after the previous newline).
func (d *Document) lineStart(pos int) int {
	i := pos
	for i > 0 && (d.source[i-1] == ' ' || d.source[i-1] == '\t') {
		i--
	}
	return i
}

// lineDeleteSpan computes the span to remove for deleting an item on its own
// line: from the start of its leading indentation through its separator and the
// rest of its line (including the trailing newline).
func (d *Document) lineDeleteSpan(itemStart, itemEnd int, sep rune) (start, length int) {
	ls := d.lineStart(itemStart)
	le := d.afterItemLine(itemEnd, sep)
	return ls, le - ls
}
