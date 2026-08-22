package setay

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/aki-kuramoto/setay-go/internal/setaypath"
)

// Document is a parsed setay source that keeps the complete original text --
// comments, whitespace, BOM, and formatting all included. It is the
// full-fidelity way to work with setay: reading a document and writing it back
// reproduces the input byte for byte, and editing one value leaves everything
// else exactly as written.
//
// This is distinct from Unmarshal/Marshal, which map to Go values in the style
// of JSON/TOML and do not preserve formatting. Use a Document when the point is
// to load a file, change a specific part, and save it with the rest intact.
//
// A Document is not safe for concurrent use while edits are being recorded.
type Document struct {
	source []rune
	root   *DefSetayDocument
	edits  []edit
}

// edit is a pending replacement of the rune span [start, start+length) in source.
type edit struct {
	start       int
	length      int
	replacement string
}

// ParseDocument parses src into a Document, retaining the original text.
func ParseDocument(src string) (*Document, error) {
	root, err := Parse(src)
	if err != nil {
		return nil, err
	}
	return &Document{source: []rune(src), root: root}, nil
}

// String returns the document's current text: the original source with any
// pending edits applied. With no edits it is byte-identical to the parsed input.
func (d *Document) String() string {
	if len(d.edits) == 0 {
		return string(d.source)
	}
	return string(d.render())
}

// Bytes is String as a byte slice.
func (d *Document) Bytes() []byte {
	return []byte(d.String())
}

// Validate re-parses the document's current text and returns any parse error.
// SetRaw does not check that the spliced-in text is valid setay; call Validate
// after raw edits when that guarantee is wanted.
func (d *Document) Validate() error {
	_, err := Parse(d.String())
	return err
}

// render applies all pending edits to a copy of the source. Edits are applied
// from the rightmost span to the leftmost, so earlier rune offsets stay valid
// as the text length changes.
func (d *Document) render() []rune {
	es := make([]edit, len(d.edits))
	copy(es, d.edits)
	sort.SliceStable(es, func(i, j int) bool { return es[i].start > es[j].start })

	out := append([]rune(nil), d.source...)
	for _, e := range es {
		repl := []rune(e.replacement)
		next := make([]rune, 0, len(out)-e.length+len(repl))
		next = append(next, out[:e.start]...)
		next = append(next, repl...)
		next = append(next, out[e.start+e.length:]...)
		out = next
	}
	return out
}

// addEdit records a replacement of the given rune span, rejecting a span that
// overlaps one already recorded (an overlap would make the result ambiguous).
func (d *Document) addEdit(start, length int, replacement string) error {
	newEnd := start + length
	for _, e := range d.edits {
		if start < e.start+e.length && e.start < newEnd {
			return fmt.Errorf("setay: edit at [%d,%d) overlaps an existing edit at [%d,%d)",
				start, newEnd, e.start, e.start+e.length)
		}
	}
	d.edits = append(d.edits, edit{start: start, length: length, replacement: replacement})
	return nil
}

// Field returns the value of a top-level dict key. The document root is always
// a dict.
func (d *Document) Field(key string) (*Node, bool) {
	return d.fieldOf(d.root.Dict, key)
}

// Get resolves a path from the document root and returns the value there.
//
// The path syntax is the single-value subset of setayq's (jq-like) filter
// syntax: it starts with '.', selects dict fields with '.name' (or '."name"' /
// '.["name"]' for keys that are not bare identifiers), and indexes lists with
// '[n]' or a negative '[-n]' from the end -- e.g. ".servers[0].host",
// `.["a.b"].port`, ".ports[-1]". Streaming/query constructs ('.[]', '|',
// functions, ',') are intentionally not accepted here; those belong to setayq.
func (d *Document) Get(path string) (*Node, bool) {
	steps, err := parsePath(path)
	if err != nil || len(steps) == 0 {
		return nil, false
	}
	var n *Node
	ok := false
	for k, st := range steps {
		switch {
		case k == 0:
			if !st.isField {
				return nil, false // the root is a dict; the first step must be a field
			}
			n, ok = d.Field(st.field)
		case st.isField:
			n, ok = n.Field(st.field)
		default:
			n, ok = n.Index(st.index)
		}
		if !ok {
			return nil, false
		}
	}
	return n, ok
}

// SetRaw replaces the value at path with raw setay text, leaving the rest of the
// document byte-identical. The text is spliced in verbatim and is not checked
// for validity (see Validate). Returns an error if the path does not resolve.
func (d *Document) SetRaw(path, setayText string) error {
	n, ok := d.Get(path)
	if !ok {
		return fmt.Errorf("setay: no value at path %q", path)
	}
	return n.SetRaw(setayText)
}

func (d *Document) fieldOf(dict *DefSetayDict, key string) (*Node, bool) {
	if dict == nil {
		return nil, false
	}
	u := &unmarshaler{source: d.source}
	for _, e := range dictEntries(dict) {
		if u.extractKeyText(e.Key) == key {
			return &Node{doc: d, value: e.Value}, true
		}
	}
	return nil, false
}

// Node is a handle to a value inside a Document. It exposes the value's original
// text and position, lets you navigate into dicts and lists, and lets you
// replace the value in place.
type Node struct {
	doc   *Document
	value *DefSetayValue
}

// Raw returns the value's original source text, exactly as written.
func (n *Node) Raw() string {
	return n.value.GetAuthority().Surface
}

// Span returns the value's position in the original source: the starting rune
// offset and its length in runes.
func (n *Node) Span() (start, length int) {
	a := n.value.GetAuthority()
	return int(a.StartedAt), int(a.Length)
}

// Kind reports the value's setay kind: one of "dict", "list", "set", "string",
// "number", "bool", "null", "utcts", "varref", or "unknown".
func (n *Node) Kind() string {
	switch n.value.AnonymousField1.(type) {
	case *DefSetayDict:
		return "dict"
	case *DefSetayList:
		return "list"
	case *DefSetaySet:
		return "set"
	case *DefSetayString:
		return "string"
	case *DefSetayNumber:
		return "number"
	case *DefSetayTrue, *DefSetayFalse:
		return "bool"
	case *DefSetayNull:
		return "null"
	case *DefSetayUtcTs:
		return "utcts"
	case *DefSetayVarRef:
		return "varref"
	default:
		return "unknown"
	}
}

// Field returns the value of the given key when this node is a dict.
func (n *Node) Field(key string) (*Node, bool) {
	dict, ok := dictOf(n.value)
	if !ok {
		return nil, false
	}
	return n.doc.fieldOf(dict, key)
}

// Index returns the i-th element when this node is a list. A negative index
// counts from the end (-1 is the last element).
func (n *Node) Index(i int) (*Node, bool) {
	list, ok := listOf(n.value)
	if !ok {
		return nil, false
	}
	vals := listValues(list)
	if i < 0 {
		i += len(vals)
	}
	if i < 0 || i >= len(vals) {
		return nil, false
	}
	return &Node{doc: n.doc, value: vals[i]}, true
}

// SetRaw replaces this value with raw setay text, leaving the rest of the
// document byte-identical. The text is spliced in verbatim and is not checked
// for validity (see Document.Validate).
func (n *Node) SetRaw(setayText string) error {
	start, length := n.Span()
	return n.doc.addEdit(start, length, setayText)
}

// Unmarshal decodes this value into v (a non-nil pointer), using the same
// mapping as the package-level Unmarshal. Useful for reading a typed value out
// of a document without giving up the round-trip fidelity of the whole.
func (n *Node) Unmarshal(v interface{}) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return fmt.Errorf("setay: Unmarshal target must be a non-nil pointer")
	}
	u := &unmarshaler{source: n.doc.source}
	return u.unmarshalValue(n.value, rv.Elem())
}

func dictOf(v *DefSetayValue) (*DefSetayDict, bool) {
	d, ok := v.AnonymousField1.(*DefSetayDict)
	return d, ok
}

func listOf(v *DefSetayValue) (*DefSetayList, bool) {
	l, ok := v.AnonymousField1.(*DefSetayList)
	return l, ok
}

func dictEntries(dict *DefSetayDict) []*DefSetayDictEntry {
	if len(dict.Entries) == 0 {
		return nil
	}
	es := dict.Entries[0]
	out := []*DefSetayDictEntry{es.First}
	for _, sep := range es.Rest {
		out = append(out, sep.Entry)
	}
	return out
}

func listValues(list *DefSetayList) []*DefSetayValue {
	if len(list.Elements) == 0 {
		return nil
	}
	return collectValues(list.Elements[0])
}

// pathStep is one hop of a Get/SetRaw path: a dict field or a list index.
type pathStep struct {
	isField bool
	field   string
	index   int
}

// parsePath parses the single-value subset of setayq's path syntax into steps.
// The grammar lives in internal/setaypath/path.bp and the parser is generated
// from it by boompaw -- there is no second, hand-written path parser to drift
// out of sync. This function only walks the generated CST into []pathStep.
func parsePath(path string) ([]pathStep, error) {
	root, err := setaypath.Parse(path)
	if err != nil {
		return nil, err
	}
	steps := make([]pathStep, 0, len(root.Steps))
	for _, st := range root.Steps {
		switch v := st.AnonymousField1.(type) {
		case *setaypath.DefSetayPathDotField:
			steps = append(steps, pathStep{isField: true, field: pathNameText(v.Name)})
		case *setaypath.DefSetayPathDotBracket:
			s, err := pathBracketStep(v.Bracket)
			if err != nil {
				return nil, err
			}
			steps = append(steps, s)
		case *setaypath.DefSetayPathBracket:
			s, err := pathBracketStep(v)
			if err != nil {
				return nil, err
			}
			steps = append(steps, s)
		default:
			return nil, fmt.Errorf("setay: unrecognized step in path %q", path)
		}
	}
	return steps, nil
}

// pathNameText returns the (decoded) key named by a ".name" or `."key"` step.
func pathNameText(name *setaypath.DefSetayPathName) string {
	switch v := name.AnonymousField1.(type) {
	case *setaypath.DefSetayPathBareName:
		return v.GetAuthority().Surface
	case *setaypath.DefSetayPathQuoted:
		return decodeQuotedKey(v)
	default:
		return name.GetAuthority().Surface
	}
}

// pathBracketStep converts a "[n]" / "[-n]" / `["key"]` bracket into a step.
func pathBracketStep(b *setaypath.DefSetayPathBracket) (pathStep, error) {
	switch v := b.Inner.AnonymousField1.(type) {
	case *setaypath.DefSetayPathIndex:
		i, err := strconv.Atoi(v.GetAuthority().Surface)
		if err != nil {
			return pathStep{}, err
		}
		return pathStep{index: i}, nil
	case *setaypath.DefSetayPathQuoted:
		return pathStep{isField: true, field: decodeQuotedKey(v)}, nil
	default:
		return pathStep{}, fmt.Errorf("setay: unrecognized bracket content in path")
	}
}

// decodeQuotedKey unescapes a double-quoted key from a path.
func decodeQuotedKey(q *setaypath.DefSetayPathQuoted) string {
	var b strings.Builder
	for _, c := range q.Chars {
		switch cc := c.AnonymousField1.(type) {
		case *setaypath.DefSetayPathEscaped:
			switch cc.Ch.Surface {
			case "n":
				b.WriteByte('\n')
			case "t":
				b.WriteByte('\t')
			case "r":
				b.WriteByte('\r')
			default:
				b.WriteString(cc.Ch.Surface)
			}
		case *setaypath.DefSetayPathQuotedNormal:
			b.WriteString(cc.GetAuthority().Surface)
		}
	}
	return b.String()
}
