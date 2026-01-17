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

const longHelpTmpl = `{{"lx" | bold }} - file discovery, slicing, and formatting tool for LLM prompting

{{ "Usage:" | head }}
  lx [OPTIONS] [state modifiers] <actions>...
  lx [OPTIONS] -- <files...>   (Disable flag parsing for subsequent arguments)

{{ "Arguments:" | head }}
  [OPTIONS]
          Global options that modify the behavior of the program.
  [state modifiers]
          asdf
  [actions]
          These are the outputs of the program. It can either be:
            1. A file, which is output and formatted directly.
		    2. A directory, which is walked recursively and processed as paths.
            3. An explicit action (see Actions section below).
          If actions is omitted the default is to walk the current working directory.
          If your 

          the search pattern which is either a regular expression (default) or a glob pattern (if
          --glob is used). If no pattern has been specified, every entry is considered a match. If
          your pattern starts with a dash (-), make sure to pass '--' first, or it will be
          considered as a flag (fd -- '-foo').

{{ "Description:" | head }}
    It recursively walks directories, respecting .gitignore rules, detects binary 
    files, and formats everything into a clean Markdown (or XML/HTML) structure 
    with token estimations.

{{ "GLOBAL OPTIONS:" | head }}
{{- range .Formatting }}
{{ . | flagLine }}
{{ .Long | wrapIndent "          " 80 }}
{{- end }}

{{- range .Config }}
{{ . | flagLine }}
{{ .Long | wrapIndent "          " 80 }}
{{- end }}

{{ "DISCOVERY OPTIONS:" | head }}
{{- range .Discovery }}
{{ . | flagLine }}
{{ .Long | wrapIndent "          " 80 }}
{{- end }}

{{ "State modifiers:" | head }}
    lx treats command line arguments as a stream. Interleaved state flags change the 
    internal state of the processor for all SUBSEQUENT files until reset.

    Think of it like a paintbrush: if you set the "color" to blue (-l), 
    everything you touch afterwards is blue until you change it.
{{- range .Interleaved }}
{{ . | flagLine }}
{{ .Long | wrapIndent "          " 80 }}
{{- end }}

{{ "ACTIONS:" | head }}
{{- range .Actions }}
{{ . | flagLine }}
{{ .Long | wrapIndent "          " 80 }}
{{- end }}

{{ "EXAMPLES:" | head }}
    1. Code review
       Copy all modified files in your git repo using 'git diff' to clipboard 
       along with your agents file and docs:
       $ git diff --name-only | lx AGENTS.md docs/ -c

    2. Debug with logs
       Grab the last 50 lines of the error log, then the processing code with 
       line numbers to help the LLM pinpoint the crash:
       $ lx -ls "Crash Log" --tail 50 /var/log/app.log \
             -LNs "Source Code" src/processor/

    3. Using filters
       Find all JavaScript files, exclude tests and node_modules, and use XML 
       format for Claude:
       $ lx --xml -i "*.js" -e "*test*" -e "node_modules" .

    4. External prompt injection
       Load a prompt from a file, add specific context, and copy:
       $ lx -p "$(cat prompt.txt)" src/main.go -c

    5. Search integration
       Use 'grep' to find files with TODOs and pipe them to lx:
       $ grep -Rl TODO | lx -c
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
			if d.Short != "" {
				sb.WriteString("-")
				sb.WriteString(d.Short)
				sb.WriteString(", ")
			} else {
				sb.WriteString("    ")
			}
			sb.WriteString("--")
			sb.WriteString(d.Name)

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
