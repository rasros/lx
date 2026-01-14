package cli

import (
	"fmt"
	"os"
	"strings"
	"text/template"
)

const helpTmpl = `NAME:
  lx - file discovery and formatting tool for LLM prompting

USAGE:
  lx [global options] [state flags] <path|action> [state flags] <path|action>...
  lx [options] -- <files...>   (Disable flag parsing for subsequent arguments)

DESCRIPTION:
  lx recursively discovers files, slices content, and formats output with
  markdown fences and headers for use with Large Language Models.

RECURSIVE DISCOVERY:
  When a directory is provided, lx walks the tree respecting .gitignore,
  .ignore, and .lxignore files. It also reads your global gitignore config 
  and global lxignore file located in ~/.config/lx/ignore.
  Binary files are automatically detected and summarized instead.

CONFIGURATION:
  Defaults can be set in ~/.config/lx/config.yaml or via the LX_CONFIG
  environment variable. Command line flags always override config values.

GLOBAL OPTIONS:
  These options control the overall behavior (output format, logging, stats).
{{ range .Globals }}
{{ . | flag }}
{{ .Usage | wrap "      " 75 }}
{{ end }}

INTERLEAVED OPTIONS (State Machine):
  lx processes arguments in order. These flags change the internal state
  (filters, line limits) and apply to all SUBSEQUENT files until changed again.
{{ range .Interleaved }}
{{ . | flag }}
{{ .Usage | wrap "      " 75 }}
{{ end }}

GLOBBING & FILTERS:
  Filters (-i/-e) use standard shell glob syntax.
  
  Logic:
  - Multiple includes (-i) are OR'ed together (match ANY include).
  - Multiple excludes (-e) are OR'ed together (match ANY exclude).
  - Excludes take precedence over includes.
  
  Syntax:
  1. Filename Match: If the pattern has no separators (e.g. "*.go"), it matches
     against the file name only, regardless of directory depth.
     > -i "*.go" matches "main.go" AND "cmd/main.go"
  
  2. Path Match: If the pattern has a separator (e.g. "cmd/*"), it matches
     against the relative path from the root.
     > -i "cmd/*.go" matches "cmd/main.go" but NOT "other/main.go"
     > -i "src/**/*.rs" matches all Rust files inside src recursively.

ACTIONS:
  These are the items processed by the stream.
{{ range .Actions }}
{{ . | flag }}
{{ .Usage | wrap "      " 75 }}
{{ end }}
  -                     Read input from stdin and format it as a single file.
                        (Default behavior without '-' is to treat stdin as a 
                        list of filenames to be discovered).

EXAMPLES:
  # Copy all code in src/, excluding tests
  lx -c -e "*_test.go" src/

  # Contextual prompt: Server code -> specific file -> instruction
  lx -s "Server" src/server -s "Config" config.yaml -p "Refactor config"

  # Find TODOs in the codebase and format them
  rg -l "TODO" | lx -c
`

func printHelp() {
	var globals, interleaved, actions []CommandDef

	for _, d := range definitions {
		if d.Internal {
			continue
		}
		switch d.Type {
		case CmdGlobal:
			globals = append(globals, d)
		case CmdInterleaved:
			interleaved = append(interleaved, d)
		case CmdAction:
			actions = append(actions, d)
		}
	}

	data := struct {
		Globals     []CommandDef
		Interleaved []CommandDef
		Actions     []CommandDef
	}{globals, interleaved, actions}

	funcs := template.FuncMap{
		"flag": func(d CommandDef) string {
			s := "  --" + d.Name
			if d.Short != "" {
				s += ", -" + d.Short
			}
			return fmt.Sprintf("%-24s", s)
		},
		"wrap": func(indent string, width int, s string) string {
			return wordWrap(s, indent, width)
		},
	}

	t := template.Must(template.New("help").Funcs(funcs).Parse(helpTmpl))
	if err := t.Execute(os.Stdout, data); err != nil {
		fmt.Fprintf(os.Stderr, "help template error: %v\n", err)
	}
}

// wordWrap breaks a string into lines of maximum 'width', prefixing subsequent
// lines with 'indent'.
func wordWrap(text string, indent string, width int) string {
	if text == "" {
		return ""
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}

	var sb strings.Builder
	// Start first line with indent
	sb.WriteString(indent)

	currentLineLen := len(indent)

	for i, word := range words {
		wordLen := len(word)

		if i == 0 {
			sb.WriteString(word)
			currentLineLen += wordLen
			continue
		}

		if currentLineLen+1+wordLen > width {
			sb.WriteString("\n")
			sb.WriteString(indent)
			sb.WriteString(word)
			currentLineLen = len(indent) + wordLen
		} else {
			sb.WriteString(" ")
			sb.WriteString(word)
			currentLineLen += 1 + wordLen
		}
	}

	return sb.String()
}
