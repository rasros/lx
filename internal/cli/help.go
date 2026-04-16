package cli

import (
	"fmt"
	"os"
	"strings"
	"text/template"
)

const (
	cResetIntensity = "\033[22m"
	cResetUnderline = "\033[24m"
	cBold           = "\033[1m"
	cDim            = "\033[2m"
	cUnderline      = "\033[4m"
)

const shortHelpTmpl = `A program to discover and format files for LLM prompting

{{ "Usage:" | head }} {{ "lx" | bold }} [OPTIONS] [path|action]...

{{ "Arguments:" | head }}
  [path]...   the file or directory to process (defaults to current dir)
  [action]... the state modifiers (flags) or prompt generators (-s, -p, -f)

{{ "Options:" | head }}
{{- range . }}
{{ . | compactFlag }}
{{- end }}
`

const longHelpTmpl = `{{"lx" | bold }} - File discovery, slicing, and formatting tool for LLM prompting.

{{ "Usage:" | head }}
  {{ "lx" | bold }} [OPTIONS] [state modifiers] <actions>...
  {{ "lx" | bold }} [OPTIONS] -- <files...>

{{ "Description:" | head }}
  lx is a specialized context bundler for Large Language Models (LLMs) like
  Claude, ChatGPT, and GitHub Copilot.

  It recursively walks directories, respecting .gitignore rules, detects binary
  files, and streams content into a clean Markdown (or XML/HTML) structure with
  token estimations.

  {{ "Stream Processing Model:" | bold }}
  lx processes arguments as a stream of sections. A section is a run of file
  and directory arguments that share the same processing settings. State
  modifiers (like --lines or --include) are scoped to the section that
  follows them. When a modifier appears {{ "after" | underline }} files, it
  creates a new section boundary: all interleaved state resets to defaults
  before the new modifier takes effect.

  Example: last 50 lines of app.log, then src/ with line numbers:
    lx --tail 50 app.log -l src/
  The -l after app.log triggers a boundary — src/ gets a fresh default state
  with only line numbers enabled.

{{ "Arguments:" | head }}
  {{ "[OPTIONS]" | bold }}
          Global flags that affect the entire execution (e.g., --copy, --xml).
          These can be placed anywhere in the command.

  {{ "[state modifiers]" | bold }}
          Flags that scope the processing rules for the next section.
          When placed after files, they reset all state before taking effect.
          Example: "-n 50 src/" limits src/ to 50 lines per file.

  {{ "[actions]" | bold }}
          The inputs to process. These can be:
            1. A file path (added directly).
            2. A directory path (walked recursively).
            3. An explicit action flag (-f, -p, -s).
            4. "-" to read file paths from stdin.

          If no actions are provided, lx walks the current directory ".".

  {{ "--" | bold }}
          Separator that disables flag parsing for subsequent arguments.
          Useful for filenames that start with a dash.

{{ "Global options:" | head }}
  Settings that apply to the whole operation, regardless of position.

{{- range .Formatting }}
{{ . | flagLine }}
{{ .Long | wrapIndent "          " 80 }}
{{- end }}

{{- range .Config }}
{{ . | flagLine }}
{{ .Long | wrapIndent "          " 80 }}
{{- end }}

{{ "Discovery options:" | head }}
  Settings that control how files are found when walking directories.

{{- range .Discovery }}
{{ . | flagLine }}
{{ .Long | wrapIndent "          " 80 }}
{{- end }}

{{ "State modifiers:" | head }}
  Flags that configure the section which follows them. When placed before any
  files, they accumulate into the first section's settings. When placed after
  files, they trigger a section boundary: all state resets to defaults, then
  the new flag applies to the next section.

{{- range .Interleaved }}
{{ . | flagLine }}
{{ .Long | wrapIndent "          " 80 }}
{{- end }}

{{ "Actions:" | head }}
  Explicit content generators that insert data into the stream immediately.

{{- range .Actions }}
{{ . | flagLine }}
{{ .Long | wrapIndent "          " 80 }}
{{- end }}

{{ "Examples:" | head }}
  1. {{ "Code Review Bundle" | underline }}
     Copy all modified files in your git repo to the clipboard:
       $ git diff --name-only | lx -c

  2. {{ "Targeted Debugging" | underline }}
     Grab the last 50 lines of a log file, then the processing code with line
     numbers enabled for reference:
       $ lx --tail 50 /var/log/app.log \
            -l src/processor/

  3. {{ "Complex Filtering" | underline }}
     Find JavaScript files except tests, output in XML (optimized for Claude):
       $ lx --xml -i "*.js" -e "*test*" .

  4. {{ "Prompt Injection" | underline }}
     Load a prompt from a file, add a separator, then the source code:
       $ lx -p "$(cat prompt.txt)" -s "Context" src/ -c

  5. {{ "Search Integration" | underline }}
     Use 'grep' to find files with TODOs and pipe them to lx:
       $ grep -Rl TODO | lx -c

Bugs can be reported on GitHub: https://github.com/rasros/lx/issues
`

func makeFuncs() template.FuncMap {
	return template.FuncMap{
		"bold":      func(s string) string { return cBold + s + cResetIntensity },
		"underline": func(s string) string { return cUnderline + s + cResetUnderline },
		"head":      func(s string) string { return cBold + cUnderline + s + cResetUnderline + cResetIntensity },

		"wrapIndent": func(indent string, width int, s string) string {
			if s == "" {
				return ""
			}
			paragraphs := strings.Split(s, "\n\n")
			var out strings.Builder

			for i, para := range paragraphs {
				if i > 0 {
					out.WriteString("\n")
					out.WriteString(indent)
					out.WriteString("\n")
				}
				cleanPara := strings.Join(strings.Fields(para), " ")
				out.WriteString(wordWrap(cleanPara, indent, width))
			}
			return out.String()
		},

		"flagLine": func(d CommandDef) string {
			var sb strings.Builder
			sb.WriteString("  ")
			sb.WriteString(cBold)
			if d.Short != "" {
				sb.WriteString("-")
				sb.WriteString(d.Short)
				sb.WriteString(", ")
			} else {
				sb.WriteString("    ")
			}
			sb.WriteString("--")
			sb.WriteString(d.Name)
			sb.WriteString(cResetIntensity)

			switch d.ValueType {
			case ValueAny:
				sb.WriteString(" <value>")
			case ValueNumber:
				sb.WriteString(" <n>")
			case ValueOptional:
				sb.WriteString(" [value]")
			}
			return sb.String()
		},

		"compactFlag": func(d CommandDef) string {
			flagPart := "  "
			if d.Short != "" {
				flagPart += "-" + d.Short + ", "
			} else {
				flagPart += "    "
			}
			flagPart += "--" + d.Name

			switch d.ValueType {
			case ValueAny:
				flagPart += " <value>"
			case ValueNumber:
				flagPart += " <n>"
			case ValueOptional:
				flagPart += " [value]"
			case ValueNone:
			}

			padding := 28
			if len(flagPart) < padding {
				flagPart += strings.Repeat(" ", padding-len(flagPart))
			} else {
				flagPart += " "
			}

			return cBold + flagPart + cResetIntensity + d.Usage
		},
	}
}

func printShortHelp() {
	var all []CommandDef
	for _, d := range definitions {
		if !d.Internal {
			all = append(all, d)
		}
	}

	t := template.Must(template.New("short-help").Funcs(makeFuncs()).Parse(shortHelpTmpl))
	if err := t.Execute(os.Stdout, all); err != nil {
		fmt.Fprintf(os.Stderr, "template error: %v\n", err)
	}
}

func printLongHelp() {
	var formatting, discovery, interleaved, actions, config []CommandDef

	for _, d := range definitions {
		if d.Internal {
			continue
		}
		switch d.Category {
		case CatFormatting:
			formatting = append(formatting, d)
		case CatDiscovery:
			discovery = append(discovery, d)
		case CatInterleaved:
			interleaved = append(interleaved, d)
		case CatActions:
			actions = append(actions, d)
		case CatConfig:
			config = append(config, d)
		}
	}

	data := struct {
		Formatting  []CommandDef
		Discovery   []CommandDef
		Interleaved []CommandDef
		Actions     []CommandDef
		Config      []CommandDef
	}{formatting, discovery, interleaved, actions, config}

	t := template.Must(template.New("long-help").Funcs(makeFuncs()).Parse(longHelpTmpl))
	if err := t.Execute(os.Stdout, data); err != nil {
		fmt.Fprintf(os.Stderr, "template error: %v\n", err)
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
