package setay

import "fmt"

// improveParseError rewrites a raw parser error into a friendlier one when the
// real problem is an unterminated string literal. The generated parser reports
// that only as a low-level "expected X, got EOF" at end of input, which never
// mentions the unclosed quote. Any other error (and a nil error) is returned
// unchanged. The returned error carries no "setay:" prefix, so each caller can
// apply its own convention.
func improveParseError(input string, err error) error {
	if err == nil {
		return nil
	}
	if q, line, col, ok := unterminatedString(input); ok {
		kind := "double-quoted"
		if q == '\'' {
			kind = "single-quoted"
		}
		return fmt.Errorf("unterminated %s string: the %c opened at line %d, column %d is never closed", kind, q, line, col)
	}
	return err
}

// unterminatedString scans input the way the grammar lexes it -- skipping
// comments and honoring backslash escapes inside strings -- and, if a quoted
// string is opened but never closed before end of input, returns that quote and
// the 1-based line/column where it opened. It is a best-effort helper used only
// to phrase error messages, so it errs toward reporting nothing (ok == false)
// rather than guessing.
func unterminatedString(input string) (quote rune, line, col int, ok bool) {
	rs := []rune(input)
	n := len(rs)
	i := 0
	for i < n {
		c := rs[i]
		switch {
		case c == '#':
			if i+1 < n && rs[i+1] == '{' {
				// Block comment #{ ... }#: skip to the closing "}#".
				i += 2
				for i+1 < n && !(rs[i] == '}' && rs[i+1] == '#') {
					i++
				}
				i += 2
			} else {
				// Line comment: skip to end of line.
				for i < n && rs[i] != '\n' {
					i++
				}
			}
		case c == '\'' || c == '"':
			open := i
			i++
			closed := false
			for i < n {
				if rs[i] == '\\' {
					i += 2 // skip an escaped character (e.g. \' or \")
					continue
				}
				if rs[i] == c {
					i++
					closed = true
					break
				}
				i++
			}
			if !closed {
				l, cc := lineColOf(rs, open)
				return c, l, cc, true
			}
		default:
			i++
		}
	}
	return 0, 0, 0, false
}

// lineColOf returns the 1-based line and column of rune offset pos.
func lineColOf(rs []rune, pos int) (line, col int) {
	line, col = 1, 1
	for i := 0; i < pos && i < len(rs); i++ {
		if rs[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return line, col
}
