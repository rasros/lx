package cli

import (
	"fmt"
	"os"
	"text/template"
)

const helpTmpl = `NAME:
   lx - print files with headers, slicing, using go-templates

USAGE:
   lx [global options] [command state options] [files/actions...]

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
   -                        format stdin as a text file

DISCOVERY:
   lx recursively walks directories found in arguments.
   It respects .gitignore, .ignore, and .lxignore files by default.
   Hidden files and directories are skipped unless -H is provided.

EXAMPLE:
   lx -n5 file1.txt -s "Section 2" -n2 file2.txt
   (Prints 5 lines of file1, a section header, then 2 lines of file2)

   lx -c src/
   (Walk src directory and copy content to clipboard)
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
