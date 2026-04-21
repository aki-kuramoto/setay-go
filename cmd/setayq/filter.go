package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"
)

// ---- Filter AST ----

type filterKind int

const (
	fIdentity filterKind = iota
	fField                // .foo
	fIndex                // .[n]
	fIter                 // .[]
	fPipe                 // a | b
	fComma                // a , b
	fOptional             // f?
	fBuiltin              // keys, length, type, select, map, not, tostring, tonumber, ascii_downcase, ascii_upcase, ltrimstr, rtrimstr, startswith, endswith, split, join, first, last, values, has, in, contains, recurse, env, path, leaf_paths, add, any, all, flatten, reverse, unique, group_by, min_by, max_by, sort_by
	fLiteral              // literal value (null, true, false, string, number)
	fArray                // [filter]
	fObject               // {key: filter, ...}
	fTry                  // try-catch
	fIf                   // if-then-else
	fBinOp                // a == b, a + b, etc.
)

type filter struct {
	kind filterKind

	// fField
	fieldName string
	optional  bool

	// fIndex
	indexVal int

	// fPipe, fComma, fBinOp
	left  *filter
	right *filter

	// fBuiltin
	builtinName string
	builtinArgs []*filter

	// fLiteral
	literal *Value

	// fIf
	cond    *filter
	thenF   *filter
	elseF   *filter

	// fArray
	inner *filter

	// fObject
	objFields []objField

	// fBinOp
	op string
}

type objField struct {
	key      string      // static key
	keyExpr  *filter     // dynamic key (.foo)
	valueExp *filter
}

// ---- Parser ----

type tokenKind int

const (
	tokDot tokenKind = iota
	tokDotDot   // ..
	tokIdent    // foo
	tokString   // "..." or 'foo-bar'
	tokNumber   // 123
	tokPipe     // |
	tokComma    // ,
	tokLBracket // [
	tokRBracket // ]
	tokLBrace   // {
	tokRBrace   // }
	tokLParen   // (
	tokRParen   // )
	tokQuestion // ?
	tokColon    // :
	tokSemicolon // ;
	tokEq       // ==
	tokNeq      // !=
	tokLt       // <
	tokGt       // >
	tokLte      // <=
	tokGte      // >=
	tokPlus     // +
	tokMinus    // -
	tokStar     // *
	tokSlash    // /
	tokPercent  // %
	tokAnd      // and
	tokOr       // or
	tokNot      // not (as keyword)
	tokNull     // null
	tokTrue     // true
	tokFalse    // false
	tokEOF
)

type token struct {
	kind tokenKind
	text string
	pos  int
}

type lexer struct {
	input []rune
	pos   int
}

func lex(input string) []token {
	l := &lexer{input: []rune(input)}
	var tokens []token
	for {
		l.skipWS()
		if l.pos >= len(l.input) {
			tokens = append(tokens, token{kind: tokEOF, pos: l.pos})
			break
		}
		tok := l.next()
		tokens = append(tokens, tok)
	}
	return tokens
}

func (l *lexer) skipWS() {
	for l.pos < len(l.input) && unicode.IsSpace(l.input[l.pos]) {
		l.pos++
	}
}

func (l *lexer) next() token {
	start := l.pos
	r := l.input[l.pos]

	switch {
	case r == '.':
		l.pos++
		if l.pos < len(l.input) && l.input[l.pos] == '.' {
			l.pos++
			return token{kind: tokDotDot, text: "..", pos: start}
		}
		return token{kind: tokDot, text: ".", pos: start}
	case r == '|':
		l.pos++
		return token{kind: tokPipe, text: "|", pos: start}
	case r == ',':
		l.pos++
		return token{kind: tokComma, text: ",", pos: start}
	case r == '[':
		l.pos++
		return token{kind: tokLBracket, text: "[", pos: start}
	case r == ']':
		l.pos++
		return token{kind: tokRBracket, text: "]", pos: start}
	case r == '{':
		l.pos++
		return token{kind: tokLBrace, text: "{", pos: start}
	case r == '}':
		l.pos++
		return token{kind: tokRBrace, text: "}", pos: start}
	case r == '(':
		l.pos++
		return token{kind: tokLParen, text: "(", pos: start}
	case r == ')':
		l.pos++
		return token{kind: tokRParen, text: ")", pos: start}
	case r == '?':
		l.pos++
		return token{kind: tokQuestion, text: "?", pos: start}
	case r == ':':
		l.pos++
		return token{kind: tokColon, text: ":", pos: start}
	case r == ';':
		l.pos++
		return token{kind: tokSemicolon, text: ";", pos: start}
	case r == '+':
		l.pos++
		return token{kind: tokPlus, text: "+", pos: start}
	case r == '-':
		l.pos++
		return token{kind: tokMinus, text: "-", pos: start}
	case r == '*':
		l.pos++
		return token{kind: tokStar, text: "*", pos: start}
	case r == '/':
		l.pos++
		return token{kind: tokSlash, text: "/", pos: start}
	case r == '%':
		l.pos++
		return token{kind: tokPercent, text: "%", pos: start}
	case r == '<':
		l.pos++
		if l.pos < len(l.input) && l.input[l.pos] == '=' {
			l.pos++
			return token{kind: tokLte, text: "<=", pos: start}
		}
		return token{kind: tokLt, text: "<", pos: start}
	case r == '>':
		l.pos++
		if l.pos < len(l.input) && l.input[l.pos] == '=' {
			l.pos++
			return token{kind: tokGte, text: ">=", pos: start}
		}
		return token{kind: tokGt, text: ">", pos: start}
	case r == '!':
		l.pos++
		if l.pos < len(l.input) && l.input[l.pos] == '=' {
			l.pos++
			return token{kind: tokNeq, text: "!=", pos: start}
		}
		return token{kind: tokIdent, text: "!", pos: start}
	case r == '=':
		l.pos++
		if l.pos < len(l.input) && l.input[l.pos] == '=' {
			l.pos++
			return token{kind: tokEq, text: "==", pos: start}
		}
		return token{kind: tokIdent, text: "=", pos: start}
	case r == '"':
		return l.lexDqString(start)
	case r == '\'':
		return l.lexSqString(start)
	case unicode.IsDigit(r) || (r == '-' && l.pos+1 < len(l.input) && unicode.IsDigit(l.input[l.pos+1])):
		return l.lexNumber(start)
	case r == '_' || unicode.IsLetter(r):
		return l.lexIdent(start)
	default:
		l.pos++
		return token{kind: tokIdent, text: string(r), pos: start}
	}
}

func (l *lexer) lexDqString(start int) token {
	l.pos++ // skip opening "
	var sb strings.Builder
	for l.pos < len(l.input) {
		r := l.input[l.pos]
		if r == '"' {
			l.pos++
			break
		}
		if r == '\\' && l.pos+1 < len(l.input) {
			l.pos++
			switch l.input[l.pos] {
			case 'n':
				sb.WriteByte('\n')
			case 't':
				sb.WriteByte('\t')
			case '"':
				sb.WriteByte('"')
			case '\\':
				sb.WriteByte('\\')
			default:
				sb.WriteRune(l.input[l.pos])
			}
			l.pos++
		} else {
			sb.WriteRune(r)
			l.pos++
		}
	}
	return token{kind: tokString, text: sb.String(), pos: start}
}

func (l *lexer) lexSqString(start int) token {
	l.pos++ // skip opening '
	var sb strings.Builder
	for l.pos < len(l.input) {
		r := l.input[l.pos]
		if r == '\'' {
			l.pos++
			break
		}
		sb.WriteRune(r)
		l.pos++
	}
	return token{kind: tokString, text: sb.String(), pos: start}
}

func (l *lexer) lexNumber(start int) token {
	if l.input[l.pos] == '-' {
		l.pos++
	}
	for l.pos < len(l.input) && unicode.IsDigit(l.input[l.pos]) {
		l.pos++
	}
	if l.pos < len(l.input) && l.input[l.pos] == '.' {
		l.pos++
		for l.pos < len(l.input) && unicode.IsDigit(l.input[l.pos]) {
			l.pos++
		}
	}
	return token{kind: tokNumber, text: string(l.input[start:l.pos]), pos: start}
}

func (l *lexer) lexIdent(start int) token {
	for l.pos < len(l.input) && (l.input[l.pos] == '_' || unicode.IsLetter(l.input[l.pos]) || unicode.IsDigit(l.input[l.pos]) || l.input[l.pos] == '-') {
		l.pos++
	}
	text := string(l.input[start:l.pos])
	switch text {
	case "and":
		return token{kind: tokAnd, text: text, pos: start}
	case "or":
		return token{kind: tokOr, text: text, pos: start}
	case "not":
		return token{kind: tokNot, text: text, pos: start}
	case "null":
		return token{kind: tokNull, text: text, pos: start}
	case "true":
		return token{kind: tokTrue, text: text, pos: start}
	case "false":
		return token{kind: tokFalse, text: text, pos: start}
	}
	return token{kind: tokIdent, text: text, pos: start}
}

// ---- Recursive-descent parser ----

type parser struct {
	tokens []token
	pos    int
}

func parseFilter(expr string) (*filter, error) {
	tokens := lex(expr)
	p := &parser{tokens: tokens}
	f, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if p.peek().kind != tokEOF {
		return nil, fmt.Errorf("unexpected token %q at position %d", p.peek().text, p.peek().pos)
	}
	return f, nil
}

func (p *parser) peek() token {
	if p.pos >= len(p.tokens) {
		return token{kind: tokEOF}
	}
	return p.tokens[p.pos]
}

func (p *parser) consume() token {
	t := p.peek()
	p.pos++
	return t
}

func (p *parser) expect(k tokenKind) (token, error) {
	t := p.peek()
	if t.kind != k {
		return t, fmt.Errorf("expected token kind %d, got %q", k, t.text)
	}
	p.pos++
	return t, nil
}

// parseExpr: comma-separated list (lowest precedence).
func (p *parser) parseExpr() (*filter, error) {
	left, err := p.parsePipe()
	if err != nil {
		return nil, err
	}
	for p.peek().kind == tokComma {
		p.consume()
		right, err := p.parsePipe()
		if err != nil {
			return nil, err
		}
		left = &filter{kind: fComma, left: left, right: right}
	}
	return left, nil
}

// parsePipe: pipe operator.
func (p *parser) parsePipe() (*filter, error) {
	left, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	for p.peek().kind == tokPipe {
		p.consume()
		right, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		left = &filter{kind: fPipe, left: left, right: right}
	}
	return left, nil
}

// parseOr / parseAnd / parseCmp / parseAdd: standard binary precedence levels.
func (p *parser) parseOr() (*filter, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.peek().kind == tokOr {
		p.consume()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &filter{kind: fBinOp, op: "or", left: left, right: right}
	}
	return left, nil
}

func (p *parser) parseAnd() (*filter, error) {
	left, err := p.parseCmp()
	if err != nil {
		return nil, err
	}
	for p.peek().kind == tokAnd {
		p.consume()
		right, err := p.parseCmp()
		if err != nil {
			return nil, err
		}
		left = &filter{kind: fBinOp, op: "and", left: left, right: right}
	}
	return left, nil
}

func (p *parser) parseCmp() (*filter, error) {
	left, err := p.parseAdd()
	if err != nil {
		return nil, err
	}
	for {
		switch p.peek().kind {
		case tokEq, tokNeq, tokLt, tokGt, tokLte, tokGte:
			op := p.consume().text
			right, err := p.parseAdd()
			if err != nil {
				return nil, err
			}
			left = &filter{kind: fBinOp, op: op, left: left, right: right}
		default:
			return left, nil
		}
	}
}

func (p *parser) parseAdd() (*filter, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for {
		switch p.peek().kind {
		case tokPlus, tokMinus:
			op := p.consume().text
			right, err := p.parseUnary()
			if err != nil {
				return nil, err
			}
			left = &filter{kind: fBinOp, op: op, left: left, right: right}
		default:
			return left, nil
		}
	}
}

func (p *parser) parseUnary() (*filter, error) {
	if p.peek().kind == tokNot {
		p.consume()
		inner, err := p.parsePostfix()
		if err != nil {
			return nil, err
		}
		return &filter{kind: fBuiltin, builtinName: "not", builtinArgs: []*filter{inner}}, nil
	}
	return p.parsePostfix()
}

// parsePostfix: handles ? optional suffix and chained field/index access.
func (p *parser) parsePostfix() (*filter, error) {
	f, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	for {
		switch p.peek().kind {
		case tokQuestion:
			p.consume()
			f = &filter{kind: fOptional, inner: f}
		case tokDot:
			// Chained: .foo.bar or .[0]
			inner, err := p.parseDotSuffix()
			if err != nil {
				return nil, err
			}
			f = &filter{kind: fPipe, left: f, right: inner}
		case tokLBracket:
			// Could be .[n] or .[] chained
			inner, err := p.parseBracketSuffix()
			if err != nil {
				return nil, err
			}
			f = &filter{kind: fPipe, left: f, right: inner}
		default:
			return f, nil
		}
	}
}

// parsePrimary: atomic expressions.
func (p *parser) parsePrimary() (*filter, error) {
	tok := p.peek()

	switch tok.kind {
	case tokDot:
		return p.parseDotExpr()
	case tokLBracket:
		return p.parseArrayConstruct()
	case tokLBrace:
		return p.parseObjectConstruct()
	case tokLParen:
		p.consume()
		inner, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(tokRParen); err != nil {
			return nil, err
		}
		return inner, nil
	case tokNull:
		p.consume()
		return &filter{kind: fLiteral, literal: &Value{kind: kindNull}}, nil
	case tokTrue:
		p.consume()
		return &filter{kind: fLiteral, literal: &Value{kind: kindBool, boolVal: true}}, nil
	case tokFalse:
		p.consume()
		return &filter{kind: fLiteral, literal: &Value{kind: kindBool, boolVal: false}}, nil
	case tokNumber:
		p.consume()
		f, _ := strconv.ParseFloat(tok.text, 64)
		return &filter{kind: fLiteral, literal: &Value{kind: kindNumber, numText: tok.text, numFloat: f}}, nil
	case tokString:
		p.consume()
		return &filter{kind: fLiteral, literal: &Value{kind: kindString, strVal: tok.text}}, nil
	case tokIdent:
		return p.parseBuiltinOrIdent()
	case tokNot:
		p.consume()
		// "not" as standalone builtin (maps input truthy → bool)
		return &filter{kind: fBuiltin, builtinName: "not", builtinArgs: nil}, nil
	}

	return nil, fmt.Errorf("unexpected token %q", tok.text)
}

func (p *parser) parseDotExpr() (*filter, error) {
	p.consume() // consume '.'

	// Check for .[..] or .[]
	if p.peek().kind == tokLBracket {
		return p.parseBracketSuffix()
	}

	// .foo or . (identity)
	if p.peek().kind == tokIdent || p.peek().kind == tokString {
		tok := p.consume()
		opt := false
		if p.peek().kind == tokQuestion {
			p.consume()
			opt = true
		}
		return &filter{kind: fField, fieldName: tok.text, optional: opt}, nil
	}

	// bare dot = identity
	return &filter{kind: fIdentity}, nil
}

// parseDotSuffix parses a chained .foo or .[n] after an existing expression.
func (p *parser) parseDotSuffix() (*filter, error) {
	p.consume() // consume '.'

	if p.peek().kind == tokLBracket {
		return p.parseBracketSuffix()
	}

	if p.peek().kind == tokIdent || p.peek().kind == tokString {
		tok := p.consume()
		opt := false
		if p.peek().kind == tokQuestion {
			p.consume()
			opt = true
		}
		return &filter{kind: fField, fieldName: tok.text, optional: opt}, nil
	}

	// Bare dot after something = pipeable identity
	return &filter{kind: fIdentity}, nil
}

func (p *parser) parseBracketSuffix() (*filter, error) {
	p.consume() // consume '['

	// .[] — iterator
	if p.peek().kind == tokRBracket {
		p.consume()
		return &filter{kind: fIter}, nil
	}

	// .[n] — index
	tok := p.peek()
	if tok.kind == tokNumber {
		p.consume()
		n, _ := strconv.Atoi(tok.text)
		if _, err := p.expect(tokRBracket); err != nil {
			return nil, err
		}
		return &filter{kind: fIndex, indexVal: n}, nil
	}

	// .["-n"] — negative index as string? Handle minus.
	if tok.kind == tokMinus {
		p.consume()
		numTok, err := p.expect(tokNumber)
		if err != nil {
			return nil, err
		}
		n, _ := strconv.Atoi(numTok.text)
		if _, err := p.expect(tokRBracket); err != nil {
			return nil, err
		}
		return &filter{kind: fIndex, indexVal: -n}, nil
	}

	// .["key"] — field access via string
	if tok.kind == tokString {
		p.consume()
		if _, err := p.expect(tokRBracket); err != nil {
			return nil, err
		}
		return &filter{kind: fField, fieldName: tok.text}, nil
	}

	return nil, fmt.Errorf("unexpected token %q in [ ]", tok.text)
}


func (p *parser) parseBuiltinOrIdent() (*filter, error) {
	tok := p.consume()
	name := tok.text

	// if-then-else
	if name == "if" {
		return p.parseIf()
	}
	// try-catch
	if name == "try" {
		return p.parseTry()
	}

	// Check for args in parens
	var args []*filter
	if p.peek().kind == tokLParen {
		p.consume()
		// parse semicolon-separated argument list
		if p.peek().kind != tokRParen {
			arg, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			args = append(args, arg)
			for p.peek().kind == tokSemicolon {
				p.consume()
				arg, err := p.parseExpr()
				if err != nil {
					return nil, err
				}
				args = append(args, arg)
			}
		}
		if _, err := p.expect(tokRParen); err != nil {
			return nil, err
		}
	}

	return &filter{kind: fBuiltin, builtinName: name, builtinArgs: args}, nil
}

func (p *parser) parseIf() (*filter, error) {
	cond, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	// expect "then"
	if p.peek().kind != tokIdent || p.peek().text != "then" {
		return nil, fmt.Errorf("expected 'then' after if condition")
	}
	p.consume()
	thenF, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	var elseF *filter
	if p.peek().kind == tokIdent && p.peek().text == "else" {
		p.consume()
		elseF, err = p.parseExpr()
		if err != nil {
			return nil, err
		}
	}
	// expect "end"
	if p.peek().kind == tokIdent && p.peek().text == "end" {
		p.consume()
	}
	return &filter{kind: fIf, cond: cond, thenF: thenF, elseF: elseF}, nil
}

func (p *parser) parseTry() (*filter, error) {
	body, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	var catch *filter
	if p.peek().kind == tokIdent && p.peek().text == "catch" {
		p.consume()
		catch, err = p.parseExpr()
		if err != nil {
			return nil, err
		}
	}
	return &filter{kind: fTry, left: body, right: catch}, nil
}

func (p *parser) parseArrayConstruct() (*filter, error) {
	p.consume() // [
	if p.peek().kind == tokRBracket {
		p.consume()
		return &filter{kind: fArray, inner: &filter{kind: fLiteral, literal: &Value{kind: kindNull}}}, nil
	}
	inner, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokRBracket); err != nil {
		return nil, err
	}
	return &filter{kind: fArray, inner: inner}, nil
}

func (p *parser) parseObjectConstruct() (*filter, error) {
	p.consume() // {
	var fields []objField
	for p.peek().kind != tokRBrace && p.peek().kind != tokEOF {
		var of objField
		tok := p.peek()
		if tok.kind == tokString || tok.kind == tokIdent {
			p.consume()
			of.key = tok.text
		} else if tok.kind == tokLParen {
			p.consume()
			expr, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(tokRParen); err != nil {
				return nil, err
			}
			of.keyExpr = expr
		} else {
			return nil, fmt.Errorf("expected key in object constructor, got %q", tok.text)
		}
		if p.peek().kind == tokColon {
			p.consume()
			valF, err := p.parsePipe()
			if err != nil {
				return nil, err
			}
			of.valueExp = valF
		} else {
			// shorthand: {foo} = {foo: .foo}
			of.valueExp = &filter{kind: fField, fieldName: of.key}
		}
		fields = append(fields, of)
		if p.peek().kind == tokComma {
			p.consume()
		}
	}
	if _, err := p.expect(tokRBrace); err != nil {
		return nil, err
	}
	return &filter{kind: fObject, objFields: fields}, nil
}

// ---- Evaluator ----

// evalFilter evaluates a filter against an input value, returning a stream of output values.
func evalFilter(f *filter, input *Value) ([]*Value, error) {
	switch f.kind {
	case fIdentity:
		return []*Value{input}, nil

	case fField:
		result, err := input.getField(f.fieldName)
		if err != nil {
			if f.optional {
				return nil, nil
			}
			return nil, err
		}
		return []*Value{result}, nil

	case fIndex:
		result, err := input.getIndex(f.indexVal)
		if err != nil {
			return nil, err
		}
		return []*Value{result}, nil

	case fIter:
		items, err := input.iterate()
		if err != nil {
			return nil, err
		}
		return items, nil

	case fPipe:
		lefts, err := evalFilter(f.left, input)
		if err != nil {
			return nil, err
		}
		var results []*Value
		for _, l := range lefts {
			rs, err := evalFilter(f.right, l)
			if err != nil {
				return nil, err
			}
			results = append(results, rs...)
		}
		return results, nil

	case fComma:
		lefts, err := evalFilter(f.left, input)
		if err != nil {
			return nil, err
		}
		rights, err := evalFilter(f.right, input)
		if err != nil {
			return nil, err
		}
		return append(lefts, rights...), nil

	case fOptional:
		results, _ := evalFilter(f.inner, input)
		return results, nil

	case fLiteral:
		return []*Value{f.literal}, nil

	case fBuiltin:
		return evalBuiltin(f, input)

	case fArray:
		results, err := evalFilter(f.inner, input)
		if err != nil {
			return nil, err
		}
		// filter out nil results from empty-producers
		var cleaned []*Value
		for _, r := range results {
			if r != nil {
				cleaned = append(cleaned, r)
			}
		}
		return []*Value{{kind: kindArray, arrVal: cleaned}}, nil

	case fObject:
		return evalObjectConstruct(f, input)

	case fBinOp:
		return evalBinOp(f, input)

	case fIf:
		conds, err := evalFilter(f.cond, input)
		if err != nil {
			return nil, err
		}
		var results []*Value
		for _, c := range conds {
			var branch *filter
			if c.truthy() {
				branch = f.thenF
			} else if f.elseF != nil {
				branch = f.elseF
			} else {
				continue
			}
			rs, err := evalFilter(branch, input)
			if err != nil {
				return nil, err
			}
			results = append(results, rs...)
		}
		return results, nil

	case fTry:
		results, err := evalFilter(f.left, input)
		if err != nil {
			if f.right != nil {
				errVal := &Value{kind: kindString, strVal: err.Error()}
				return evalFilter(f.right, errVal)
			}
			return nil, nil
		}
		return results, nil
	}

	return nil, fmt.Errorf("unhandled filter kind %d", f.kind)
}

func evalBuiltin(f *filter, input *Value) ([]*Value, error) {
	switch f.builtinName {
	case "keys":
		v, err := input.keys(true)
		if err != nil {
			return nil, err
		}
		return []*Value{v}, nil

	case "keys_unsorted":
		v, err := input.keys(false)
		if err != nil {
			return nil, err
		}
		return []*Value{v}, nil

	case "values":
		switch input.kind {
		case kindDict:
			vals := make([]*Value, len(input.dictKeys))
			for i, k := range input.dictKeys {
				vals[i] = input.dictVals[k]
			}
			return []*Value{{kind: kindArray, arrVal: vals}}, nil
		case kindArray:
			return []*Value{{kind: kindArray, arrVal: input.arrVal}}, nil
		default:
			return nil, fmt.Errorf("values requires object or array, got %s", input.typeName())
		}

	case "length":
		n, err := input.length()
		if err != nil {
			return nil, err
		}
		return []*Value{{kind: kindNumber, numText: strconv.Itoa(n), numFloat: float64(n)}}, nil

	case "type":
		return []*Value{{kind: kindString, strVal: input.typeName()}}, nil

	case "not":
		if len(f.builtinArgs) > 0 {
			results, err := evalFilter(f.builtinArgs[0], input)
			if err != nil {
				return nil, err
			}
			var out []*Value
			for _, r := range results {
				out = append(out, &Value{kind: kindBool, boolVal: !r.truthy()})
			}
			return out, nil
		}
		return []*Value{{kind: kindBool, boolVal: !input.truthy()}}, nil

	case "select":
		if len(f.builtinArgs) == 0 {
			return nil, fmt.Errorf("select requires an argument")
		}
		results, err := evalFilter(f.builtinArgs[0], input)
		if err != nil {
			return nil, nil // select silently drops errors
		}
		for _, r := range results {
			if r.truthy() {
				return []*Value{input}, nil
			}
		}
		return nil, nil

	case "map":
		if len(f.builtinArgs) == 0 {
			return nil, fmt.Errorf("map requires an argument")
		}
		items, err := input.iterate()
		if err != nil {
			return nil, err
		}
		var out []*Value
		for _, item := range items {
			rs, err := evalFilter(f.builtinArgs[0], item)
			if err != nil {
				return nil, err
			}
			out = append(out, rs...)
		}
		return []*Value{{kind: kindArray, arrVal: out}}, nil

	case "tostring":
		if input.kind == kindString {
			return []*Value{input}, nil
		}
		s := valueToSetayString(input)
		return []*Value{{kind: kindString, strVal: s}}, nil

	case "tonumber":
		if input.kind == kindNumber {
			return []*Value{input}, nil
		}
		if input.kind == kindString {
			f, err := strconv.ParseFloat(input.strVal, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid number: %q", input.strVal)
			}
			return []*Value{{kind: kindNumber, numText: input.strVal, numFloat: f}}, nil
		}
		return nil, fmt.Errorf("cannot convert %s to number", input.typeName())

	case "ascii_downcase":
		if input.kind != kindString {
			return nil, fmt.Errorf("ascii_downcase requires string, got %s", input.typeName())
		}
		return []*Value{{kind: kindString, strVal: strings.ToLower(input.strVal)}}, nil

	case "ascii_upcase":
		if input.kind != kindString {
			return nil, fmt.Errorf("ascii_upcase requires string, got %s", input.typeName())
		}
		return []*Value{{kind: kindString, strVal: strings.ToUpper(input.strVal)}}, nil

	case "ltrimstr":
		if len(f.builtinArgs) == 0 {
			return nil, fmt.Errorf("ltrimstr requires an argument")
		}
		args, err := evalFilter(f.builtinArgs[0], input)
		if err != nil || len(args) == 0 {
			return []*Value{input}, nil
		}
		prefix := args[0].strVal
		if input.kind == kindString {
			return []*Value{{kind: kindString, strVal: strings.TrimPrefix(input.strVal, prefix)}}, nil
		}
		return []*Value{input}, nil

	case "rtrimstr":
		if len(f.builtinArgs) == 0 {
			return nil, fmt.Errorf("rtrimstr requires an argument")
		}
		args, err := evalFilter(f.builtinArgs[0], input)
		if err != nil || len(args) == 0 {
			return []*Value{input}, nil
		}
		suffix := args[0].strVal
		if input.kind == kindString {
			return []*Value{{kind: kindString, strVal: strings.TrimSuffix(input.strVal, suffix)}}, nil
		}
		return []*Value{input}, nil

	case "startswith":
		if len(f.builtinArgs) == 0 {
			return nil, fmt.Errorf("startswith requires an argument")
		}
		args, err := evalFilter(f.builtinArgs[0], input)
		if err != nil || len(args) == 0 {
			return nil, err
		}
		ok := input.kind == kindString && strings.HasPrefix(input.strVal, args[0].strVal)
		return []*Value{{kind: kindBool, boolVal: ok}}, nil

	case "endswith":
		if len(f.builtinArgs) == 0 {
			return nil, fmt.Errorf("endswith requires an argument")
		}
		args, err := evalFilter(f.builtinArgs[0], input)
		if err != nil || len(args) == 0 {
			return nil, err
		}
		ok := input.kind == kindString && strings.HasSuffix(input.strVal, args[0].strVal)
		return []*Value{{kind: kindBool, boolVal: ok}}, nil

	case "split":
		if len(f.builtinArgs) == 0 {
			return nil, fmt.Errorf("split requires an argument")
		}
		args, err := evalFilter(f.builtinArgs[0], input)
		if err != nil || len(args) == 0 {
			return nil, err
		}
		if input.kind != kindString {
			return nil, fmt.Errorf("split requires string input")
		}
		parts := strings.Split(input.strVal, args[0].strVal)
		arr := make([]*Value, len(parts))
		for i, p := range parts {
			arr[i] = &Value{kind: kindString, strVal: p}
		}
		return []*Value{{kind: kindArray, arrVal: arr}}, nil

	case "join":
		if len(f.builtinArgs) == 0 {
			return nil, fmt.Errorf("join requires an argument")
		}
		args, err := evalFilter(f.builtinArgs[0], input)
		if err != nil || len(args) == 0 {
			return nil, err
		}
		sep := args[0].strVal
		if input.kind != kindArray {
			return nil, fmt.Errorf("join requires array input")
		}
		var parts []string
		for _, item := range input.arrVal {
			parts = append(parts, item.strVal)
		}
		return []*Value{{kind: kindString, strVal: strings.Join(parts, sep)}}, nil

	case "has":
		if len(f.builtinArgs) == 0 {
			return nil, fmt.Errorf("has requires an argument")
		}
		args, err := evalFilter(f.builtinArgs[0], &Value{kind: kindNull})
		if err != nil || len(args) == 0 {
			return nil, err
		}
		key := args[0].strVal
		switch input.kind {
		case kindDict:
			_, ok := input.dictVals[key]
			return []*Value{{kind: kindBool, boolVal: ok}}, nil
		case kindSet:
			for _, k := range input.setKeys {
				if k == key {
					return []*Value{{kind: kindBool, boolVal: true}}, nil
				}
			}
			return []*Value{{kind: kindBool, boolVal: false}}, nil
		default:
			return nil, fmt.Errorf("has requires object or set, got %s", input.typeName())
		}

	case "reverse":
		if input.kind != kindArray {
			return nil, fmt.Errorf("reverse requires array, got %s", input.typeName())
		}
		n := len(input.arrVal)
		rev := make([]*Value, n)
		for i, v := range input.arrVal {
			rev[n-1-i] = v
		}
		return []*Value{{kind: kindArray, arrVal: rev}}, nil

	case "flatten":
		if input.kind != kindArray {
			return nil, fmt.Errorf("flatten requires array, got %s", input.typeName())
		}
		flat := flattenArray(input.arrVal)
		return []*Value{{kind: kindArray, arrVal: flat}}, nil

	case "first":
		if len(f.builtinArgs) > 0 {
			results, err := evalFilter(f.builtinArgs[0], input)
			if err != nil {
				return nil, err
			}
			if len(results) == 0 {
				return []*Value{{kind: kindNull}}, nil
			}
			return []*Value{results[0]}, nil
		}
		if input.kind != kindArray || len(input.arrVal) == 0 {
			return []*Value{{kind: kindNull}}, nil
		}
		return []*Value{input.arrVal[0]}, nil

	case "last":
		if len(f.builtinArgs) > 0 {
			results, err := evalFilter(f.builtinArgs[0], input)
			if err != nil {
				return nil, err
			}
			if len(results) == 0 {
				return []*Value{{kind: kindNull}}, nil
			}
			return []*Value{results[len(results)-1]}, nil
		}
		if input.kind != kindArray || len(input.arrVal) == 0 {
			return []*Value{{kind: kindNull}}, nil
		}
		return []*Value{input.arrVal[len(input.arrVal)-1]}, nil

	case "empty":
		return nil, nil

	case "add":
		if input.kind != kindArray {
			return []*Value{{kind: kindNull}}, nil
		}
		if len(input.arrVal) == 0 {
			return []*Value{{kind: kindNull}}, nil
		}
		acc := input.arrVal[0]
		for _, v := range input.arrVal[1:] {
			var err error
			acc, err = addValues(acc, v)
			if err != nil {
				return nil, err
			}
		}
		return []*Value{acc}, nil

	case "unique":
		if input.kind != kindArray {
			return nil, fmt.Errorf("unique requires array")
		}
		seen := map[string]bool{}
		var out []*Value
		for _, v := range input.arrVal {
			key := valueToSetayString(v)
			if !seen[key] {
				seen[key] = true
				out = append(out, v)
			}
		}
		return []*Value{{kind: kindArray, arrVal: out}}, nil

	case "env":
		// Return environment as a dict
		v := &Value{kind: kindDict, dictVals: make(map[string]*Value)}
		for _, e := range os.Environ() {
			parts := strings.SplitN(e, "=", 2)
			if len(parts) == 2 {
				v.dictKeys = append(v.dictKeys, parts[0])
				v.dictVals[parts[0]] = &Value{kind: kindString, strVal: parts[1]}
			}
		}
		return []*Value{v}, nil

	case "to_entries":
		if input.kind != kindDict {
			return nil, fmt.Errorf("to_entries requires object, got %s", input.typeName())
		}
		var arr []*Value
		for _, k := range input.dictKeys {
			entry := &Value{
				kind:     kindDict,
				dictKeys: []string{"name", "value"},
				dictVals: map[string]*Value{
					"name":  {kind: kindString, strVal: k},
					"value": input.dictVals[k],
				},
			}
			arr = append(arr, entry)
		}
		return []*Value{{kind: kindArray, arrVal: arr}}, nil

	case "from_entries":
		if input.kind != kindArray {
			return nil, fmt.Errorf("from_entries requires array, got %s", input.typeName())
		}
		result := &Value{kind: kindDict, dictVals: make(map[string]*Value)}
		for _, entry := range input.arrVal {
			if entry.kind != kindDict {
				continue
			}
			// Accept "name"/"key" and "value"
			var key string
			if v, ok := entry.dictVals["name"]; ok {
				key = v.strVal
			} else if v, ok := entry.dictVals["key"]; ok {
				key = v.strVal
			}
			val := entry.dictVals["value"]
			if key != "" && val != nil {
				if _, exists := result.dictVals[key]; !exists {
					result.dictKeys = append(result.dictKeys, key)
				}
				result.dictVals[key] = val
			}
		}
		return []*Value{result}, nil

	case "with_entries":
		if len(f.builtinArgs) == 0 {
			return nil, fmt.Errorf("with_entries requires an argument")
		}
		// to_entries | map(f) | from_entries
		entries, err := evalBuiltin(&filter{kind: fBuiltin, builtinName: "to_entries"}, input)
		if err != nil {
			return nil, err
		}
		mapFilter := &filter{kind: fBuiltin, builtinName: "map", builtinArgs: f.builtinArgs}
		mapped, err := evalBuiltin(mapFilter, entries[0])
		if err != nil {
			return nil, err
		}
		return evalBuiltin(&filter{kind: fBuiltin, builtinName: "from_entries"}, mapped[0])

	case "sort":
		if input.kind != kindArray {
			return nil, fmt.Errorf("sort requires array, got %s", input.typeName())
		}
		sorted := make([]*Value, len(input.arrVal))
		copy(sorted, input.arrVal)
		sortValues(sorted)
		return []*Value{{kind: kindArray, arrVal: sorted}}, nil

	case "sort_by":
		if len(f.builtinArgs) == 0 || input.kind != kindArray {
			return nil, fmt.Errorf("sort_by requires an argument and array input")
		}
		sorted := make([]*Value, len(input.arrVal))
		copy(sorted, input.arrVal)
		sortValuesByFilter(sorted, f.builtinArgs[0])
		return []*Value{{kind: kindArray, arrVal: sorted}}, nil

	case "group_by":
		if len(f.builtinArgs) == 0 || input.kind != kindArray {
			return nil, fmt.Errorf("group_by requires an argument and array input")
		}
		type group struct {
			key string
			val *Value
			arr []*Value
		}
		var groups []group
		keyMap := map[string]int{}
		for _, item := range input.arrVal {
			ks, _ := evalFilter(f.builtinArgs[0], item)
			keyStr := ""
			if len(ks) > 0 {
				keyStr = valueToSetayString(ks[0])
			}
			if idx, ok := keyMap[keyStr]; ok {
				groups[idx].arr = append(groups[idx].arr, item)
			} else {
				var keyVal *Value
				if len(ks) > 0 {
					keyVal = ks[0]
				} else {
					keyVal = &Value{kind: kindNull}
				}
				keyMap[keyStr] = len(groups)
				groups = append(groups, group{key: keyStr, val: keyVal, arr: []*Value{item}})
			}
		}
		out := make([]*Value, len(groups))
		for i, g := range groups {
			out[i] = &Value{kind: kindArray, arrVal: g.arr}
		}
		return []*Value{{kind: kindArray, arrVal: out}}, nil

	case "min_by", "max_by":
		if len(f.builtinArgs) == 0 || input.kind != kindArray {
			return nil, fmt.Errorf("%s requires an argument and array input", f.builtinName)
		}
		if len(input.arrVal) == 0 {
			return []*Value{{kind: kindNull}}, nil
		}
		isMin := f.builtinName == "min_by"
		best := input.arrVal[0]
		bestKeys, _ := evalFilter(f.builtinArgs[0], best)
		for _, item := range input.arrVal[1:] {
			ks, _ := evalFilter(f.builtinArgs[0], item)
			if len(ks) == 0 || len(bestKeys) == 0 {
				continue
			}
			cmp := compareValues(ks[0], bestKeys[0])
			if (isMin && cmp < 0) || (!isMin && cmp > 0) {
				best = item
				bestKeys = ks
			}
		}
		return []*Value{best}, nil

	case "min", "max":
		if input.kind != kindArray || len(input.arrVal) == 0 {
			return []*Value{{kind: kindNull}}, nil
		}
		best := input.arrVal[0]
		isMin := f.builtinName == "min"
		for _, v := range input.arrVal[1:] {
			cmp := compareValues(v, best)
			if (isMin && cmp < 0) || (!isMin && cmp > 0) {
				best = v
			}
		}
		return []*Value{best}, nil

	case "error":
		msg := "error"
		if len(f.builtinArgs) > 0 {
			rs, _ := evalFilter(f.builtinArgs[0], input)
			if len(rs) > 0 {
				msg = rs[0].strVal
			}
		} else if input.kind == kindString {
			msg = input.strVal
		}
		return nil, fmt.Errorf("%s", msg)

	case "debug":
		// Print to stderr and pass through
		fmt.Fprintf(os.Stderr, "[debug] %s\n", valueToSetayString(input))
		return []*Value{input}, nil

	case "tojson":
		opts := outputOptions{toJSON: true, indent: "  "}
		b, err := valueToJSONBytes(input, opts)
		if err != nil {
			return nil, err
		}
		return []*Value{{kind: kindString, strVal: string(b)}}, nil

	case "fromjson":
		if input.kind != kindString {
			return nil, fmt.Errorf("fromjson requires string, got %s", input.typeName())
		}
		v, err := jsonToValue(input.strVal)
		if err != nil {
			return nil, err
		}
		return []*Value{v}, nil

	case "recurse":
		if len(f.builtinArgs) > 0 {
			// recurse until error or empty
			var results []*Value
			var walk func(v *Value) error
			walk = func(v *Value) error {
				results = append(results, v)
				rs, err := evalFilter(f.builtinArgs[0], v)
				if err != nil {
					return nil
				}
				for _, r := range rs {
					if err := walk(r); err != nil {
						return err
					}
				}
				return nil
			}
			_ = walk(input)
			return results, nil
		}
		// Default: recurse into all nested values
		var results []*Value
		var walk func(v *Value)
		walk = func(v *Value) {
			results = append(results, v)
			switch v.kind {
			case kindArray:
				for _, item := range v.arrVal {
					walk(item)
				}
			case kindDict:
				for _, k := range v.dictKeys {
					walk(v.dictVals[k])
				}
			}
		}
		walk(input)
		return results, nil

	case "any":
		items, err := input.iterate()
		if err != nil {
			return nil, err
		}
		if len(f.builtinArgs) > 0 {
			for _, item := range items {
				rs, err := evalFilter(f.builtinArgs[0], item)
				if err != nil {
					continue
				}
				for _, r := range rs {
					if r.truthy() {
						return []*Value{{kind: kindBool, boolVal: true}}, nil
					}
				}
			}
		} else {
			for _, item := range items {
				if item.truthy() {
					return []*Value{{kind: kindBool, boolVal: true}}, nil
				}
			}
		}
		return []*Value{{kind: kindBool, boolVal: false}}, nil

	case "all":
		items, err := input.iterate()
		if err != nil {
			return nil, err
		}
		if len(f.builtinArgs) > 0 {
			for _, item := range items {
				rs, err := evalFilter(f.builtinArgs[0], item)
				if err != nil {
					return []*Value{{kind: kindBool, boolVal: false}}, nil
				}
				for _, r := range rs {
					if !r.truthy() {
						return []*Value{{kind: kindBool, boolVal: false}}, nil
					}
				}
			}
		} else {
			for _, item := range items {
				if !item.truthy() {
					return []*Value{{kind: kindBool, boolVal: false}}, nil
				}
			}
		}
		return []*Value{{kind: kindBool, boolVal: true}}, nil

	case "range":
		var from, to, step float64
		step = 1
		if len(f.builtinArgs) == 1 {
			rs, _ := evalFilter(f.builtinArgs[0], input)
			if len(rs) > 0 {
				to = rs[0].numFloat
			}
		} else if len(f.builtinArgs) >= 2 {
			rs0, _ := evalFilter(f.builtinArgs[0], input)
			rs1, _ := evalFilter(f.builtinArgs[1], input)
			if len(rs0) > 0 {
				from = rs0[0].numFloat
			}
			if len(rs1) > 0 {
				to = rs1[0].numFloat
			}
			if len(f.builtinArgs) >= 3 {
				rs2, _ := evalFilter(f.builtinArgs[2], input)
				if len(rs2) > 0 {
					step = rs2[0].numFloat
				}
			}
		}
		var out []*Value
		for i := from; i < to; i += step {
			out = append(out, &Value{kind: kindNumber, numFloat: i, numText: strconv.FormatFloat(i, 'g', -1, 64)})
		}
		return out, nil
	}

	// Unknown builtin — return input unchanged (graceful degradation)
	return []*Value{input}, nil
}

func evalObjectConstruct(f *filter, input *Value) ([]*Value, error) {
	result := &Value{kind: kindDict, dictVals: make(map[string]*Value)}
	for _, of := range f.objFields {
		var key string
		if of.keyExpr != nil {
			ks, err := evalFilter(of.keyExpr, input)
			if err != nil {
				return nil, err
			}
			if len(ks) == 0 {
				continue
			}
			key = ks[0].strVal
		} else {
			key = of.key
		}
		vals, err := evalFilter(of.valueExp, input)
		if err != nil {
			return nil, err
		}
		var val *Value
		if len(vals) > 0 {
			val = vals[0]
		} else {
			val = &Value{kind: kindNull}
		}
		if _, exists := result.dictVals[key]; !exists {
			result.dictKeys = append(result.dictKeys, key)
		}
		result.dictVals[key] = val
	}
	return []*Value{result}, nil
}

func evalBinOp(f *filter, input *Value) ([]*Value, error) {
	lefts, err := evalFilter(f.left, input)
	if err != nil {
		return nil, err
	}
	rights, err := evalFilter(f.right, input)
	if err != nil {
		return nil, err
	}
	if len(lefts) == 0 || len(rights) == 0 {
		return nil, nil
	}
	l, r := lefts[0], rights[0]
	switch f.op {
	case "==":
		return []*Value{{kind: kindBool, boolVal: valuesEqual(l, r)}}, nil
	case "!=":
		return []*Value{{kind: kindBool, boolVal: !valuesEqual(l, r)}}, nil
	case "<":
		return []*Value{{kind: kindBool, boolVal: compareValues(l, r) < 0}}, nil
	case ">":
		return []*Value{{kind: kindBool, boolVal: compareValues(l, r) > 0}}, nil
	case "<=":
		return []*Value{{kind: kindBool, boolVal: compareValues(l, r) <= 0}}, nil
	case ">=":
		return []*Value{{kind: kindBool, boolVal: compareValues(l, r) >= 0}}, nil
	case "and":
		return []*Value{{kind: kindBool, boolVal: l.truthy() && r.truthy()}}, nil
	case "or":
		return []*Value{{kind: kindBool, boolVal: l.truthy() || r.truthy()}}, nil
	case "+":
		v, err := addValues(l, r)
		if err != nil {
			return nil, err
		}
		return []*Value{v}, nil
	case "-":
		if l.kind == kindNumber && r.kind == kindNumber {
			res := l.numFloat - r.numFloat
			return []*Value{{kind: kindNumber, numFloat: res, numText: strconv.FormatFloat(res, 'g', -1, 64)}}, nil
		}
		return nil, fmt.Errorf("cannot subtract %s from %s", r.typeName(), l.typeName())
	case "*":
		if l.kind == kindNumber && r.kind == kindNumber {
			res := l.numFloat * r.numFloat
			return []*Value{{kind: kindNumber, numFloat: res, numText: strconv.FormatFloat(res, 'g', -1, 64)}}, nil
		}
		return nil, fmt.Errorf("cannot multiply %s by %s", l.typeName(), r.typeName())
	case "/":
		if l.kind == kindNumber && r.kind == kindNumber {
			if r.numFloat == 0 {
				return nil, fmt.Errorf("division by zero")
			}
			res := l.numFloat / r.numFloat
			return []*Value{{kind: kindNumber, numFloat: res, numText: strconv.FormatFloat(res, 'g', -1, 64)}}, nil
		}
		return nil, fmt.Errorf("cannot divide %s by %s", l.typeName(), r.typeName())
	case "%":
		if l.kind == kindNumber && r.kind == kindNumber {
			res := float64(int64(l.numFloat) % int64(r.numFloat))
			return []*Value{{kind: kindNumber, numFloat: res, numText: strconv.FormatFloat(res, 'g', -1, 64)}}, nil
		}
		return nil, fmt.Errorf("cannot modulo %s by %s", l.typeName(), r.typeName())
	}
	return nil, fmt.Errorf("unknown operator: %s", f.op)
}

func addValues(l, r *Value) (*Value, error) {
	switch {
	case l.kind == kindNull:
		return r, nil
	case r.kind == kindNull:
		return l, nil
	case l.kind == kindNumber && r.kind == kindNumber:
		res := l.numFloat + r.numFloat
		return &Value{kind: kindNumber, numFloat: res, numText: strconv.FormatFloat(res, 'g', -1, 64)}, nil
	case l.kind == kindString && r.kind == kindString:
		return &Value{kind: kindString, strVal: l.strVal + r.strVal}, nil
	case l.kind == kindArray && r.kind == kindArray:
		arr := make([]*Value, len(l.arrVal)+len(r.arrVal))
		copy(arr, l.arrVal)
		copy(arr[len(l.arrVal):], r.arrVal)
		return &Value{kind: kindArray, arrVal: arr}, nil
	case l.kind == kindDict && r.kind == kindDict:
		result := &Value{kind: kindDict, dictVals: make(map[string]*Value)}
		for _, k := range l.dictKeys {
			result.dictKeys = append(result.dictKeys, k)
			result.dictVals[k] = l.dictVals[k]
		}
		for _, k := range r.dictKeys {
			if _, exists := result.dictVals[k]; !exists {
				result.dictKeys = append(result.dictKeys, k)
			}
			result.dictVals[k] = r.dictVals[k]
		}
		return result, nil
	}
	return nil, fmt.Errorf("cannot add %s and %s", l.typeName(), r.typeName())
}

func valuesEqual(l, r *Value) bool {
	if l.kind != r.kind {
		return false
	}
	switch l.kind {
	case kindNull:
		return true
	case kindBool:
		return l.boolVal == r.boolVal
	case kindNumber:
		return l.numFloat == r.numFloat
	case kindString:
		return l.strVal == r.strVal
	case kindArray:
		if len(l.arrVal) != len(r.arrVal) {
			return false
		}
		for i := range l.arrVal {
			if !valuesEqual(l.arrVal[i], r.arrVal[i]) {
				return false
			}
		}
		return true
	case kindDict:
		if len(l.dictKeys) != len(r.dictKeys) {
			return false
		}
		for _, k := range l.dictKeys {
			rv, ok := r.dictVals[k]
			if !ok || !valuesEqual(l.dictVals[k], rv) {
				return false
			}
		}
		return true
	}
	return false
}

func compareValues(l, r *Value) int {
	if l.kind == kindNumber && r.kind == kindNumber {
		if l.numFloat < r.numFloat {
			return -1
		}
		if l.numFloat > r.numFloat {
			return 1
		}
		return 0
	}
	ls := valueToSetayString(l)
	rs := valueToSetayString(r)
	if ls < rs {
		return -1
	}
	if ls > rs {
		return 1
	}
	return 0
}

func flattenArray(arr []*Value) []*Value {
	var result []*Value
	for _, v := range arr {
		if v.kind == kindArray {
			result = append(result, flattenArray(v.arrVal)...)
		} else {
			result = append(result, v)
		}
	}
	return result
}

// sortValues sorts a slice of *Value in-place by their natural representation.
func sortValues(arr []*Value) {
	n := len(arr)
	for i := 1; i < n; i++ {
		for j := i; j > 0 && compareValues(arr[j], arr[j-1]) < 0; j-- {
			arr[j], arr[j-1] = arr[j-1], arr[j]
		}
	}
}

func sortValuesByFilter(arr []*Value, f *filter) {
	// insertion sort for simplicity
	n := len(arr)
	for i := 1; i < n; i++ {
		for j := i; j > 0; j-- {
			lk, _ := evalFilter(f, arr[j])
			rk, _ := evalFilter(f, arr[j-1])
			if len(lk) == 0 || len(rk) == 0 {
				break
			}
			if compareValues(lk[0], rk[0]) < 0 {
				arr[j], arr[j-1] = arr[j-1], arr[j]
			} else {
				break
			}
		}
	}
}
