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
  .ignore, and .lxignore files.
  
  These ignore files are loaded from **every directory** traversed, meaning rules
  in subdirectories override or augment rules from parent directories (just like git).

  It also reads your global gitignore config and global lxignore file 
  located in ~/.config/lx/ignore.
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
  lx processes arguments sequentially from left to right. These flags change the
  internal "state" of the processor and apply to all SUBSEQUENT files/paths 
  until they are explicitly changed or reset.

  Think of it like a paintbrush: if you set the "color" to blue, all subsequent
  files are painted blue until you pick up red.

  Example:
    lx --line-numbers file1.go --no-line-numbers file2.go
    > file1.go will have line numbers.
    > file2.go will be clean.

{{ range .Interleaved }}
{{ . | flag }}
{{ .Usage | wrap "      " 75 }}
{{ end }}

ACTIONS:
  These are the items processed by the stream.
{{ range .Actions }}
{{ . | flag }}
{{ .Usage | wrap "      " 75 }}
{{ end }}
  -           
      Read input from stdin and format it as a single file.
      (Default behavior without '-' is to treat stdin as a  list of 
      filenames to be discovered).


SECTIONS & GROUPING:
  Sections allow you to logically group files in the output. This is useful
  for providing context to the LLM (e.g., separating "Database" code from 
  "UI" code).

  Effects of using --section / -s:
  1. Header: Inserts a Markdown H2 or XML <section> tag.
  2. Counters: Resets the file progress counter (e.g., [1/5]) for that section.
  3. XML Structure: Wraps the contained files in <section>...</section> tags
     when using --xml output.

  Example:
    lx -s "Backend" cmd/ api/ -s "Frontend" src/ui/


GLOBBING & FILTERS:
  Filters (-i/-e) use standard shell glob syntax. Use quotes to prevent your
  shell from expanding them before lx sees them.
  
  Logic:
  - Multiple includes (-i) are OR'ed together (match ANY include).
  - Multiple excludes (-e) are OR'ed together (match ANY exclude).
  - Excludes take precedence over includes.
  
  Supported Syntax:
  - * Matches any sequence of non-separator characters.
             e.g., "*.go" matches "main.go"
  - ?        Matches any single non-separator character.
             e.g., "test?.js" matches "test1.js" but not "test10.js"
  - [abc]    Matches any character in the set.
  - [a-z]    Matches any character in the range.
  - ** Matches directories recursively (zero or more).
             e.g., "src/**/*.css" matches "src/a.css" and "src/styles/b.css"

  Matching Strategy:
  1. Filename Match: If the pattern has no separators (e.g. "*.go"), it matches
     against the file name only, regardless of directory depth.
     > -i "*.go" matches "main.go" AND "cmd/main.go"
  
  2. Path Match: If the pattern has a separator (e.g. "cmd/*"), it matches
     against the relative path from the root.
     > -i "cmd/*.go" matches "cmd/main.go" but NOT "other/main.go"



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

func wordWrap(text string, indent string, width int) string {
	if text == "" {
		return ""
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}

	var sb strings.Builder
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
