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
  [action]... interleaved options and action flags; only -s starts a new section,
              resetting interleaved options to their defaults

{{ "Options:" | head }}
{{ range $i, $g := . }}{{ if $i }}

{{ end }}  {{ $g | groupHead }}
{{- range $g.Flags }}
{{ . | compactFlag }}
{{- end }}
{{- end }}
`

const longHelpTmpl = `{{"lx" | bold }} - File discovery, slicing, and formatting tool for LLM prompting.

{{ "Usage:" | head }}
  {{ "lx" | bold }} [OPTIONS] [interleaved options] <actions>...
  {{ "lx" | bold }} [OPTIONS] -- <files...>

{{ "Description:" | head }}
  lx is a specialized context bundler for Large Language Models (LLMs) like
  Claude, ChatGPT, and GitHub Copilot.

  It recursively walks directories, respecting .gitignore rules, detects binary
  files, and streams content into Markdown, XML/HTML, or bare text with
  token estimations.

  {{ "Stream Processing Model:" | bold }}
  lx processes arguments as a stream of sections. A section is a run of file
  and directory arguments that share the same processing settings. Interleaved
  options (like --lines or --include) are scoped to the section that follows
  them.

  Section boundaries are created in two ways:
    1. An interleaved option appears {{ "after" | underline }} files: all interleaved options
       reset to defaults before the new option takes effect.
    2. A {{ "-s" | bold }} (section) action is used: this also resets all interleaved
       options to defaults for the files that follow.

  Example: last 50 lines of app.log, then src/ with line numbers:
    lx --tail 50 app.log -l src/
  The -l after app.log triggers a boundary; src/ gets a fresh default state
  with only line numbers enabled.

  Example: two named sections with different settings:
    lx -s "Backend" -l src/ -s "Frontend" -i "*.ts" web/
  Each -s resets interleaved options; web/ gets only the -i filter.

{{ "Arguments:" | head }}
  {{ "[OPTIONS]" | bold }}
          Global options that affect the entire execution (e.g., --copy, --xml, --bare).
          These can be placed anywhere in the command.

  {{ "[interleaved options]" | bold }}
          Options that scope the processing rules for the next section.
          When placed after files, they reset all options before taking effect.
          Example: "-n 50 src/" limits src/ to 50 lines per file.

  {{ "[actions]" | bold }}
          The inputs to process. These can be:
            1. A file path (added directly).
            2. A directory path (walked recursively).
            3. An http/https URL (fetched and added as a file).
            4. An explicit action flag (-f, -p, -s).
            5. "-" to read file paths from stdin.

          Archive URLs (.zip, .tar.gz, etc.) are expanded when -Z is active.

          If no actions are provided, lx walks the current directory ".".

  {{ "--" | bold }}
          Separator that disables flag parsing for subsequent arguments.
          Useful for filenames that start with a dash.

{{ "Global option:" | head }}
  Settings that apply to the whole operation, regardless of position.
{{ range .Formatting }}
{{ . | flagLine }}
{{ .Long | wrapIndent "          " 80 }}
{{- end }}
{{ range .Config }}
{{ . | flagLine }}
{{ .Long | wrapIndent "          " 80 }}
{{- end }}

{{ "Interleaved option:" | head }}
  Options that configure the section which follows them. When placed before any
  files, they accumulate into the first section's settings. When placed after
  files, they trigger a section boundary: all interleaved options reset to
  defaults, then the new option applies to the next section.
{{ range .Interleaved }}
{{ . | flagLine }}
{{ .Long | wrapIndent "          " 80 }}
{{- end }}

{{ "Action:" | head }}
  Explicit content generators that insert data into the stream immediately.
{{ range .Actions }}
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
     Load a prompt by name from your prompts library (set via $LX_PROMPTS_DIR
     or --prompts-dir), then attach the source code:
       $ lx -P go/test src/ -c
     Use -P with an explicit path for one-off prompt files:
       $ lx -P ./prompt.txt -s "Context" src/ -c
     Use -p only for inline literals:
       $ lx -p "Refactor for concurrency:" main.go

  5. {{ "Search Integration" | underline }}
     Use 'grep' to find files with TODOs and pipe them to lx:
       $ grep -Rl TODO | lx -c

  6. {{ "URL Input" | underline }}
     Fetch a remote file directly (no download step needed):
       $ lx https://example.com/config.yaml src/

     Expand a GitHub archive and filter to Go files only:
       $ lx -Z -i "*.go" https://github.com/owner/repo/archive/refs/heads/main.zip

Bugs can be reported on GitHub: https://github.com/rasros/lx/issues
`

type flagGroup struct {
	TypeName string
	Flags    []CommandDef
}

func makeFuncs() template.FuncMap {
	return template.FuncMap{
		"bold":      func(s string) string { return cBold + s + cResetIntensity },
		"underline": func(s string) string { return cUnderline + s + cResetUnderline },
		"head":      func(s string) string { return cBold + cUnderline + s + cResetUnderline + cResetIntensity },
		"dim":       func(s string) string { return cDim + s + cResetIntensity },
		"groupHead": func(g flagGroup) string {
			return cUnderline + g.TypeName + cResetUnderline
		},

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
				lines := strings.Split(strings.TrimRight(para, "\n"), "\n")
				firstBlock := true
				j := 0
				for j < len(lines) {
					line := lines[j]
					trimmed := strings.TrimLeft(line, " \t")
					relIndent := line[:len(line)-len(trimmed)]
					if relIndent != "" {
						if !firstBlock {
							out.WriteString("\n")
						}
						if trimmed == "" {
							out.WriteString(indent)
						} else {
							out.WriteString(wordWrap(trimmed, indent+relIndent, width))
						}
						firstBlock = false
						j++
					} else {
						var words []string
						for j < len(lines) {
							l := lines[j]
							t := strings.TrimLeft(l, " \t")
							if l[:len(l)-len(t)] != "" {
								break
							}
							words = append(words, strings.Fields(t)...)
							j++
						}
						if len(words) > 0 {
							if !firstBlock {
								out.WriteString("\n")
							}
							out.WriteString(wordWrap(strings.Join(words, " "), indent, width))
							firstBlock = false
						}
					}
				}
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
	groupOrder := []struct {
		cmdType  CmdType
		typeName string
	}{
		{CmdGlobal, "Global option:"},
		{CmdInterleaved, "Interleaved option:"},
		{CmdAction, "Action:"},
	}
	byType := make(map[CmdType][]CommandDef)
	for _, d := range definitions {
		if !d.Internal {
			byType[d.Type] = append(byType[d.Type], d)
		}
	}
	var groups []flagGroup
	for _, g := range groupOrder {
		if flags := byType[g.cmdType]; len(flags) > 0 {
			groups = append(groups, flagGroup{TypeName: g.typeName, Flags: flags})
		}
	}

	t := template.Must(template.New("short-help").Funcs(makeFuncs()).Parse(shortHelpTmpl))
	if err := t.Execute(os.Stdout, groups); err != nil {
		fmt.Fprintf(os.Stderr, "template error: %v\n", err)
	}
}

func printLongHelp() {
	var formatting, interleaved, actions, config []CommandDef

	for _, d := range definitions {
		if d.Internal {
			continue
		}
		switch d.Category {
		case CatFormatting:
			formatting = append(formatting, d)
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
		Interleaved []CommandDef
		Actions     []CommandDef
		Config      []CommandDef
	}{formatting, interleaved, actions, config}

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
