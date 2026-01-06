package cli

import (
	"fmt"
	"os"
	"text/template"
)

const helpTmpl = `NAME:
   lx - file discovery and formatting tool for LLM prompting

SYNOPSIS:
   lx [options] [files/globs]

DESCRIPTION:
   lx recursively discovers files, slices content, and formats output with 
   markdown fences and headers for use with Large Language Models.

   It integrates .gitignore logic, binary file detection, and token estimation 
   into a single stream suitable for piping or clipboard copy.

GLOBAL OPTIONS:
{{- range .Globals }}
   --{{ .Name | printf "%-16s" }}{{ if .Short }}-{{ .Short | printf "%-4s" }}{{ else }}     {{ end }} {{ .Usage }}
{{- end }}

INTERLEAVED OPTIONS (apply to subsequent files):
{{- range .Interleaved }}
   --{{ .Name | printf "%-16s" }}{{ if .Short }}-{{ .Short | printf "%-4s" }}{{ else }}     {{ end }} {{ .Usage }}
{{- end }}

ACTIONS (printed in order):
{{- range .Actions }}
   --{{ .Name | printf "%-16s" }}{{ if .Short }}-{{ .Short | printf "%-4s" }}{{ else }}     {{ end }} {{ .Usage }}
{{- end }}
                     -     format input read from stdin

EXAMPLES:
   lx -c src/                         # Copy all non-ignored files in src/
   lx -n50 server.log                 # Print first/last 25 lines of log
   lx -p "Refactor:" file.go          # Prepend instruction to file content
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

	t := template.Must(template.New("help").Parse(helpTmpl))
	if err := t.Execute(os.Stdout, data); err != nil {
		fmt.Fprintf(os.Stderr, "help template error: %v\n", err)
	}
}
