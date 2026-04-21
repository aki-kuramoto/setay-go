// Package main implements the setayq CLI tool —
// a jq-like query and transformation tool for setay-format data.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	setay "github.com/aki-kuramoto/setay-go"
)

const usage = `Usage: setayq [options] <filter> [file...]

Query and transform setay-format structured data, similar to jq.

If no file is given, or the file is "-", input is read from stdin.

Options:
  -c, --compact-output     Output on a single line
  -r, --raw-output         Output strings without quotes
  -n, --null-input         Use null as input (do not read files)
  -S, --sort-keys          Sort dict keys alphabetically
      --to-json            Output in JSON format instead of setay
      --resolve-vars       Resolve ${VAR} references from environment
      --tab                Indent with tab (default)
      --indent <n>         Indent with n spaces
  -e, --exit-status        Exit with non-zero if no output or false/null

Filter examples:
  .                        Identity (prints the whole document)
  .name                    Access field "name"
  .servers[0]              First element of "servers" list
  .servers[]               Iterate all elements
  .name,.host              Output multiple values
  .servers[] | .host       Pipeline: iterate and access field
  keys                     List keys of a dict/set
  length                   Length of a string/list/dict
  type                     Type of the value
  select(.enabled)         Filter: pass through if truthy
`

func main() {
	var (
		compact     bool
		rawOutput   bool
		nullInput   bool
		sortKeys    bool
		toJSON      bool
		resolveVars bool
		useTab      bool
		indentN     int
		exitStatus  bool
		fromFile    string
	)

	fs := flag.NewFlagSet("setayq", flag.ContinueOnError)
	fs.BoolVar(&compact, "c", false, "compact output")
	fs.BoolVar(&compact, "compact-output", false, "compact output")
	fs.BoolVar(&rawOutput, "r", false, "raw output")
	fs.BoolVar(&rawOutput, "raw-output", false, "raw output")
	fs.BoolVar(&nullInput, "n", false, "null input")
	fs.BoolVar(&nullInput, "null-input", false, "null input")
	fs.BoolVar(&sortKeys, "S", false, "sort keys")
	fs.BoolVar(&sortKeys, "sort-keys", false, "sort keys")
	fs.BoolVar(&toJSON, "to-json", false, "output as JSON")
	fs.BoolVar(&resolveVars, "resolve-vars", false, "resolve ${VAR} from environment")
	fs.BoolVar(&useTab, "tab", false, "indent with tab")
	fs.IntVar(&indentN, "indent", -1, "indent with n spaces")
	fs.BoolVar(&exitStatus, "e", false, "exit with non-zero on false/null output")
	fs.BoolVar(&exitStatus, "exit-status", false, "exit with non-zero on false/null output")
	fs.StringVar(&fromFile, "f", "", "read filter from file")
	fs.StringVar(&fromFile, "from-file", "", "read filter from file")

	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }

	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}

	args := fs.Args()
	if len(args) == 0 && fromFile == "" {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	// Determine indent string
	indentStr := "\t"
	if indentN >= 0 {
		indentStr = strings.Repeat(" ", indentN)
	} else if useTab {
		indentStr = "\t"
	}

	// Load filter expression
	var filterExpr string
	if fromFile != "" {
		data, err := os.ReadFile(fromFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "setayq: cannot read filter file %s: %v\n", fromFile, err)
			os.Exit(2)
		}
		filterExpr = strings.TrimSpace(string(data))
		// files are the remaining args
	} else {
		filterExpr = args[0]
		args = args[1:]
	}

	// Parse the filter
	filter, err := parseFilter(filterExpr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "setayq: invalid filter: %v\n", err)
		os.Exit(3)
	}

	outputOpts := outputOptions{
		compact:   compact,
		rawOutput: rawOutput,
		sortKeys:  sortKeys,
		toJSON:    toJSON,
		indent:    indentStr,
	}

	// Determine input sources
	var inputs []string
	if nullInput {
		inputs = nil // will use null as input
	} else if len(args) == 0 || (len(args) == 1 && args[0] == "-") {
		inputs = []string{"-"}
	} else {
		inputs = args
	}

	hasOutput := false
	lastOutputFalsy := false

	processInput := func(r io.Reader) {
		data, err := io.ReadAll(r)
		if err != nil {
			fmt.Fprintf(os.Stderr, "setayq: read error: %v\n", err)
			os.Exit(1)
		}
		val, err := parseSetayToValue(string(data), resolveVars)
		if err != nil {
			fmt.Fprintf(os.Stderr, "setayq: parse error: %v\n", err)
			os.Exit(3)
		}

		results, evalErr := evalFilter(filter, val)
		if evalErr != nil {
			fmt.Fprintf(os.Stderr, "setayq: %v\n", evalErr)
			os.Exit(5)
		}

		for _, result := range results {
			hasOutput = true
			lastOutputFalsy = isFalsy(result)
			if err := printValue(result, outputOpts); err != nil {
				fmt.Fprintf(os.Stderr, "setayq: output error: %v\n", err)
				os.Exit(1)
			}
		}
	}

	if nullInput {
		// Feed null value as input
		results, evalErr := evalFilter(filter, &Value{kind: kindNull})
		if evalErr != nil {
			fmt.Fprintf(os.Stderr, "setayq: %v\n", evalErr)
			os.Exit(5)
		}
		for _, result := range results {
			hasOutput = true
			lastOutputFalsy = isFalsy(result)
			if err := printValue(result, outputOpts); err != nil {
				fmt.Fprintf(os.Stderr, "setayq: output error: %v\n", err)
				os.Exit(1)
			}
		}
	} else {
		for _, input := range inputs {
			if input == "-" {
				processInput(os.Stdin)
			} else {
				f, err := os.Open(input)
				if err != nil {
					fmt.Fprintf(os.Stderr, "setayq: cannot open %s: %v\n", input, err)
					os.Exit(2)
				}
				processInput(f)
				f.Close()
			}
		}
	}

	if exitStatus && (!hasOutput || lastOutputFalsy) {
		os.Exit(5)
	}
}

// parseSetayToValue reads raw setay text and converts it to an AST-based Value.
func parseSetayToValue(source string, resolveVars bool) (*Value, error) {
	if resolveVars {
		setay.RegisterVariableResolver(nil) // use env vars only
	}
	doc, err := setay.Parse(source)
	if err != nil {
		return nil, err
	}
	return astDictToValue(doc.Dict, []rune(source)), nil
}

// isFalsy returns true if the value is null or boolean false.
func isFalsy(v *Value) bool {
	if v == nil || v.kind == kindNull {
		return true
	}
	if v.kind == kindBool && !v.boolVal {
		return true
	}
	return false
}

// printValue formats and prints a single value to stdout.
func printValue(v *Value, opts outputOptions) error {
	if opts.toJSON {
		return printJSON(v, opts)
	}
	return printSetay(v, opts)
}

func printJSON(v *Value, opts outputOptions) error {
	raw, err := valueToJSON(v, opts)
	if err != nil {
		return err
	}
	var out []byte
	if opts.compact {
		// re-compact
		var m interface{}
		if err := json.Unmarshal(raw, &m); err == nil {
			out, err = json.Marshal(m)
			if err != nil {
				return err
			}
		} else {
			out = raw
		}
	} else {
		out = raw
	}
	fmt.Println(string(out))
	return nil
}
