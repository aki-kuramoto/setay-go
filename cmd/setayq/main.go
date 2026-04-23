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
  -c, --compact-output        Output on a single line
  -r, --raw-output            Output strings without quotes
  -n, --null-input            Use null as input (do not read files)
  -s, --slurp                 Read all inputs into an array before filtering
  -S, --sort-keys             Sort dict keys alphabetically
      --to-json               Output in JSON format instead of setay
      --output-set-as-object  JSON output: render Set as {"key":true} instead of ["key"]
      --resolve-vars          Resolve ${VAR} references from environment
      --tab                   Indent with tab (default)
      --indent <n>            Indent with n spaces
  -e, --exit-status           Exit with non-zero if no output or false/null
  -C, --color-output          Colorize output (default when stdout is a TTY)
  -M, --monochrome-output     Disable colorized output
      --arg <name> <value>    Bind string value to $name in filter
      --argjson <name> <val>  Bind JSON-parsed value to $name in filter
  -f, --from-file <file>      Read filter from file

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
  $myvar                   Reference a variable bound with --arg
`

func main() {
	var (
		compact      bool
		rawOutput    bool
		nullInput    bool
		slurp        bool
		sortKeys     bool
		toJSON       bool
		setAsObject  bool
		resolveVars  bool
		useTab       bool
		indentN      int
		exitStatus   bool
		colorOutput  bool
		monoOutput   bool
		fromFile     string
	)

	// Pre-scan os.Args for --arg / --argjson before flag.Parse,
	// because flag does not support repeated key-value pair flags.
	argsMap := make(map[string]*Value)
	cleanArgs := []string{}
	rawArgs := os.Args[1:]
	for i := 0; i < len(rawArgs); i++ {
		switch rawArgs[i] {
		case "--arg":
			if i+2 < len(rawArgs) {
				name := rawArgs[i+1]
				val := rawArgs[i+2]
				argsMap[name] = &Value{kind: kindString, strVal: val}
				i += 2
			}
		case "--argjson":
			if i+2 < len(rawArgs) {
				name := rawArgs[i+1]
				raw := rawArgs[i+2]
				v, err := jsonToValue(raw)
				if err != nil {
					fmt.Fprintf(os.Stderr, "setayq: --argjson %s: invalid JSON: %v\n", name, err)
					os.Exit(2)
				}
				argsMap[name] = v
				i += 2
			}
		default:
			cleanArgs = append(cleanArgs, rawArgs[i])
		}
	}

	fs := flag.NewFlagSet("setayq", flag.ContinueOnError)
	fs.BoolVar(&compact, "c", false, "compact output")
	fs.BoolVar(&compact, "compact-output", false, "compact output")
	fs.BoolVar(&rawOutput, "r", false, "raw output")
	fs.BoolVar(&rawOutput, "raw-output", false, "raw output")
	fs.BoolVar(&nullInput, "n", false, "null input")
	fs.BoolVar(&nullInput, "null-input", false, "null input")
	fs.BoolVar(&slurp, "s", false, "slurp")
	fs.BoolVar(&slurp, "slurp", false, "slurp")
	fs.BoolVar(&sortKeys, "S", false, "sort keys")
	fs.BoolVar(&sortKeys, "sort-keys", false, "sort keys")
	fs.BoolVar(&toJSON, "to-json", false, "output as JSON")
	fs.BoolVar(&setAsObject, "output-set-as-object", false, "render Set as object in JSON output")
	fs.BoolVar(&resolveVars, "resolve-vars", false, "resolve ${VAR} from environment")
	fs.BoolVar(&useTab, "tab", false, "indent with tab")
	fs.IntVar(&indentN, "indent", -1, "indent with n spaces")
	fs.BoolVar(&exitStatus, "e", false, "exit with non-zero on false/null output")
	fs.BoolVar(&exitStatus, "exit-status", false, "exit with non-zero on false/null output")
	fs.BoolVar(&colorOutput, "C", false, "color output")
	fs.BoolVar(&colorOutput, "color-output", false, "color output")
	fs.BoolVar(&monoOutput, "M", false, "monochrome output")
	fs.BoolVar(&monoOutput, "monochrome-output", false, "monochrome output")
	fs.StringVar(&fromFile, "f", "", "read filter from file")
	fs.StringVar(&fromFile, "from-file", "", "read filter from file")

	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }

	if err := fs.Parse(cleanArgs); err != nil {
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

	// Determine color: auto-detect TTY unless overridden
	useColor := isTTY(os.Stdout)
	if colorOutput {
		useColor = true
	}
	if monoOutput {
		useColor = false
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
	} else {
		filterExpr = args[0]
		args = args[1:]
	}

	// Parse the filter
	fil, err := parseFilter(filterExpr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "setayq: invalid filter: %v\n", err)
		os.Exit(3)
	}

	outputOpts := outputOptions{
		compact:     compact,
		rawOutput:   rawOutput,
		sortKeys:    sortKeys,
		toJSON:      toJSON,
		setAsObject: setAsObject,
		indent:      indentStr,
		color:       useColor,
	}

	ctx := evalContext{args: argsMap}

	// Determine input sources
	var inputs []string
	if nullInput {
		inputs = nil
	} else if len(args) == 0 || (len(args) == 1 && args[0] == "-") {
		inputs = []string{"-"}
	} else {
		inputs = args
	}

	hasOutput := false
	lastOutputFalsy := false

	printResults := func(results []*Value) {
		for _, result := range results {
			hasOutput = true
			lastOutputFalsy = isFalsy(result)
			if err := printValue(result, outputOpts); err != nil {
				fmt.Fprintf(os.Stderr, "setayq: output error: %v\n", err)
				os.Exit(1)
			}
		}
	}

	readAndParse := func(r io.Reader) *Value {
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
		return val
	}

	openInput := func(name string) (*os.File, func()) {
		if name == "-" {
			return os.Stdin, func() {}
		}
		f, err := os.Open(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "setayq: cannot open %s: %v\n", name, err)
			os.Exit(2)
		}
		return f, func() { f.Close() }
	}

	if nullInput {
		results, evalErr := evalFilter(fil, &Value{kind: kindNull}, ctx)
		if evalErr != nil {
			fmt.Fprintf(os.Stderr, "setayq: %v\n", evalErr)
			os.Exit(5)
		}
		printResults(results)
	} else if slurp {
		// Collect all parsed values into one array
		var collected []*Value
		for _, input := range inputs {
			f, done := openInput(input)
			val := readAndParse(f)
			done()
			collected = append(collected, val)
		}
		slurped := &Value{kind: kindArray, arrVal: collected}
		results, evalErr := evalFilter(fil, slurped, ctx)
		if evalErr != nil {
			fmt.Fprintf(os.Stderr, "setayq: %v\n", evalErr)
			os.Exit(5)
		}
		printResults(results)
	} else {
		for _, input := range inputs {
			f, done := openInput(input)
			val := readAndParse(f)
			done()
			results, evalErr := evalFilter(fil, val, ctx)
			if evalErr != nil {
				fmt.Fprintf(os.Stderr, "setayq: %v\n", evalErr)
				os.Exit(5)
			}
			printResults(results)
		}
	}

	if exitStatus && (!hasOutput || lastOutputFalsy) {
		os.Exit(5)
	}
}

// isTTY reports whether f is connected to a terminal.
func isTTY(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
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
