package cli

import (
	"fmt"
	"os"
	"strings"
	"text/template"
)

// ANSI Color Codes
const (
	// Reset codes specific to modes
	cResetIntensity = "\033[22m"
	cResetUnderline = "\033[24m"
	// Styles
	cBold      = "\033[1m"
	cDim       = "\033[2m"
	cUnderline = "\033[4m"
)

// -- Short Help Template (fd-style) --
const shortHelpTmpl = `
A program to discover and format files for LLM prompting

{{ "Usage:" | head }} {{ "lx" | bold }} [OPTIONS] [path|action]...

{{ "Arguments:" | head }}
  [path]...   File or directory to process (defaults to current dir)
  [action]... State modifiers (flags) or prompt generators (-s, -p, -f)

{{ "Options:" | head }}
{{- range . }}
{{ . | compactFlag }}
{{- end }}
`

// -- Long Help Template (fd-style) --
const longHelpTmpl = `{{ "NAME:" | head }}
    lx - file discovery, slicing, and formatting tool for LLM prompting

{{ "USAGE:" | head }}
    lx [global options] [state flags] <path|action> [state flags] <path|action>...
    lx [options] -- <files...>   (Disable flag parsing for subsequent arguments)

{{ "DESCRIPTION:" | head }}
    lx is designed to package code and text for Large Language Models (LLMs). 
    It acts as a bridge between your filesystem and your AI chat context.

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

{{ "INTERLEAVED OPTIONS (The Paintbrush):" | head }}
    lx treats command line arguments as a stream. Interleaved flags change the 
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

// Helper to safely wrap string manipulation
func makeFuncs() template.FuncMap {
	return template.FuncMap{
		"bold":      func(s string) string { return cBold + s + cResetIntensity },
		"underline": func(s string) string { return cUnderline + s + cResetUnderline },
		"head":      func(s string) string { return cBold + cUnderline + s + cResetUnderline + cResetIntensity },

		// For Long Help Description Wrapping
		"wrapIndent": func(indent string, width int, s string) string {
			if s == "" {
				return ""
			}
			// Split by double newline to preserve paragraphs
			paragraphs := strings.Split(s, "\n\n")
			var out strings.Builder

			for i, para := range paragraphs {
				if i > 0 {
					out.WriteString("\n")
					out.WriteString(indent) // empty line between paragraphs needs indent if preserving structure, or just \n
					out.WriteString("\n")
				}
				// Normalize newlines within a paragraph to spaces
				cleanPara := strings.Join(strings.Fields(para), " ")
				out.WriteString(wordWrap(cleanPara, indent, width))
			}
			return out.String()
		},

		// For Long Help Flag Line (e.g., " -c, --copy")
		"flagLine": func(d CommandDef) string {
			// Format: " -s, --long <val>"
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

		// For Short Help (fd style)
		"compactFlag": func(d CommandDef) string {
			// Format: " -s, --long <val>      Description"

			// 1. Build the flag part
			flagPart := "  "
			if d.Short != "" {
				flagPart += "-" + d.Short + ", "
			} else {
				flagPart += "    "
			}
			flagPart += "--" + d.Name

			// 2. Add value hint
			switch d.ValueType {
			case ValueAny:
				flagPart += " <value>"
			case ValueNumber:
				flagPart += " <n>"
			case ValueOptional:
				flagPart += " [value]"
			}

			// 3. Pad to align descriptions
			padding := 26
			if len(flagPart) < padding {
				flagPart += strings.Repeat(" ", padding-len(flagPart))
			} else {
				flagPart += " "
			}

			// 4. Wrap description if needed
			desc := wordWrap(d.Usage, strings.Repeat(" ", padding), 80)
			// Remove the initial indent from wordWrap since we append it to flagPart
			desc = strings.TrimLeft(desc, " ")

			return flagPart + desc
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
