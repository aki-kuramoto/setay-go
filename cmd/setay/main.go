package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// noBackup is set to true when the -w flag is specified.
var noBackup bool

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: setay <subcommand> [options] [args...]")
		fmt.Fprintln(os.Stderr, "Subcommands:")
		fmt.Fprintln(os.Stderr, "  fmt [-w] <file-or-dir> [...]   Format .setay files")
		fmt.Fprintln(os.Stderr, "Options:")
		fmt.Fprintln(os.Stderr, "  -w   Write in-place without creating .bak backup (default: create backup)")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "fmt":
		args := os.Args[2:]

		// Parse flags
		var targets []string
		for _, arg := range args {
			if arg == "-w" {
				noBackup = true
			} else {
				targets = append(targets, arg)
			}
		}

		if len(targets) == 0 {
			fmt.Fprintln(os.Stderr, "Usage: setay fmt [-w] <file-or-dir> [...]")
			fmt.Fprintln(os.Stderr, "  -w   Write in-place without creating .bak backup")
			os.Exit(1)
		}

		exitCode := 0
		for _, arg := range targets {
			if err := fmtTarget(arg); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				exitCode = 1
			}
		}
		os.Exit(exitCode)
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n", os.Args[1])
		os.Exit(1)
	}
}

// fmtTarget formats a file or recursively formats all .setay files in a directory.
func fmtTarget(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && strings.HasSuffix(d.Name(), ".setay") {
				fmtFile(p)
			}
			return nil
		})
	}

	fmtFile(path)
	return nil
}

// fmtFile formats a single .setay file. Parse errors are reported to stderr.
func fmtFile(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: cannot read %s: %v\n", path, err)
		return
	}

	source := string(data)
	doc, parseErr := Parse(source)
	if parseErr != nil {
		fmt.Fprintf(os.Stderr, "warning: parse error in %s: %v\n", path, parseErr)
		return
	}

	// Check if all input was consumed
	runes := []rune(source)
	start := int(doc.Authority.StartedAt)
	length := int(doc.Authority.Length)
	if start+length != len(runes) {
		fmt.Fprintf(os.Stderr, "warning: parse incomplete in %s (consumed %d of %d characters)\n",
			path, start+length, len(runes))
		return
	}

	formatted := Format(source, doc)

	// Only write if content changed
	if formatted == source {
		return
	}

	// Create backup unless -w is specified
	if !noBackup {
		bakPath := path + ".bak"
		if err := os.WriteFile(bakPath, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "warning: cannot create backup %s: %v\n", bakPath, err)
			return
		}
	}

	if err := os.WriteFile(path, []byte(formatted), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "warning: cannot write %s: %v\n", path, err)
	}
}
