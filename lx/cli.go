package lx

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"
	"strconv"
	"text/template"
)

var Version = "(devel)"

func init() {
	if Version != "(devel)" {
		return
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		v := info.Main.Version
		if v != "" && v != "(devel)" {
			Version = v
		}
	}
}

var definitions = []CommandDef{
	{Name: "config", Short: "y", Type: CmdGlobal, ValueType: ValueAny, Usage: "path to yaml config file"},
	{Name: "version", Short: "V", Type: CmdGlobal, ValueType: ValueNone, Usage: "print the version"},
	{Name: "help", Short: "h", Type: CmdGlobal, ValueType: ValueNone, Usage: "show help"},

	// Toggles
	{Name: "line-numbers", Short: "l", Type: CmdGlobal, ValueType: ValueNone, Usage: "print line numbers"},
	{Name: "no-line-numbers", Short: "L", Type: CmdGlobal, ValueType: ValueNone, Usage: "don't print line numbers"},

	// Interleaved - State
	{Name: "head", Type: CmdInterleaved, ValueType: ValueNumber, Usage: "print first N lines (0 = no limit)"},
	{Name: "tail", Type: CmdInterleaved, ValueType: ValueNumber, Usage: "print last N lines (0 = no limit)"},
	{Name: "n", Short: "n", Type: CmdInterleaved, ValueType: ValueNumber, Usage: "print N lines split between head and tail"},

	// Interleaved - Actions
	{Name: "section", Short: "s", Type: CmdInterleaved, ValueType: ValueAny, Usage: "print a section header"},
	{Name: "prompt", Short: "p", Type: CmdInterleaved, ValueType: ValueAny, Usage: "print custom text directly"},
}

func Run(ctx context.Context, args []string) error {
	parsed, err := Parse(args, definitions)
	if err != nil {
		return err
	}

	if done := handleGlobals(parsed); done {
		return nil
	}

	if len(args) == 0 {
		printHelp()
		return nil
	}

	if err := gatherInputs(parsed); err != nil {
		return err
	}

	return processStream(parsed)
}

func handleGlobals(parsed *ParsedArgs) bool {
	if _, ok := parsed.Globals["help"]; ok {
		printHelp()
		return true
	}
	if _, ok := parsed.Globals["version"]; ok {
		fmt.Printf("lx version %s\n", Version)
		return true
	}
	return false
}

func gatherInputs(parsed *ParsedArgs) error {
	hasFilesOrGenerators := false
	for _, op := range parsed.Ops {
		if op.Action == "FILE" || op.Action == "section" || op.Action == "prompt" {
			hasFilesOrGenerators = true
			break
		}
	}

	if !hasFilesOrGenerators {
		stdinFiles, err := readFilenamesFromStdin()
		if err != nil {
			return fmt.Errorf("read stdin: %w", err)
		}
		for _, f := range stdinFiles {
			parsed.Ops = append(parsed.Ops, Op{Action: "FILE", Value: f})
			hasFilesOrGenerators = true
		}
	}

	if !hasFilesOrGenerators {
		return fmt.Errorf("no input files provided")
	}
	return nil
}

func processStream(parsed *ParsedArgs) error {
	var opts Options
	if cfg, ok := parsed.Globals["config"]; ok {
		opts.ConfigPath = cfg
	}

	if _, ok := parsed.Globals["no-line-numbers"]; ok {
		opts.LineNumbers = false
	}
	if _, ok := parsed.Globals["line-numbers"]; ok {
		opts.LineNumbers = true
	}

	ops := reorderTrailingOps(parsed.Ops)

	// --- Pass 1: Validate all inputs ---
	// We strictly check that all files exist before printing a single byte.
	for _, op := range ops {
		if op.Action == "FILE" {
			if _, err := os.Stat(op.Value); err != nil {
				return fmt.Errorf("stat %q: %w", op.Value, err)
			}
		}
	}

	// --- Pass 2: Execute and Print ---
	totalFiles := 0
	for _, op := range ops {
		if op.Action == "FILE" {
			totalFiles++
		}
	}

	fileIndex := 1
	for _, op := range ops {
		switch op.Action {
		case "FILE":
			runner, err := opts.Effective()
			if err != nil {
				return err
			}
			if err := runner.RunFile(op.Value, fileIndex, totalFiles, os.Stdout); err != nil {
				return err
			}
			fileIndex++

		case "section":
			runner, err := opts.Effective()
			if err != nil {
				return err
			}
			if err := runner.RunSection(op.Value, os.Stdout); err != nil {
				return err
			}

		case "prompt":
			runner, err := opts.Effective()
			if err != nil {
				return err
			}
			if err := runner.RunPrompt(op.Value, os.Stdout); err != nil {
				return err
			}

		case "head":
			val, _ := strconv.Atoi(op.Value)
			opts.Head = val
			opts.HeadSet = true
			opts.Tail, opts.TailSet = 0, false
			opts.NBoth, opts.NSet = 0, false

		case "tail":
			val, _ := strconv.Atoi(op.Value)
			opts.Tail = val
			opts.TailSet = true
			opts.Head, opts.HeadSet = 0, false
			opts.NBoth, opts.NSet = 0, false

		case "n":
			val, _ := strconv.Atoi(op.Value)
			opts.NBoth = val
			opts.NSet = true
			opts.HeadSet = false
			opts.TailSet = false
		}
	}
	return nil
}

// ... (reorderTrailingOps and helpTmpl/printHelp remain the same) ...
func reorderTrailingOps(ops []Op) []Op {
	lastFileIdx := -1
	for i := len(ops) - 1; i >= 0; i-- {
		if ops[i].Action == "FILE" {
			lastFileIdx = i
			break
		}
	}

	if lastFileIdx == -1 || lastFileIdx == len(ops)-1 {
		return ops
	}

	newOps := make([]Op, 0, len(ops))
	newOps = append(newOps, ops[:lastFileIdx]...)
	newOps = append(newOps, ops[lastFileIdx+1:]...)
	newOps = append(newOps, ops[lastFileIdx])

	return newOps
}

const helpTmpl = `NAME:
   lx - print files with headers, slicing, and go-templates

USAGE:
   lx [global options] [command state options] [files...]

GLOBAL OPTIONS:
{{- range .Globals }}
   --{{ .Name | printf "%-16s" }}{{ if .Short }}-{{ .Short | printf "%-4s" }}{{ else }}      {{ end }} {{ .Usage }}
{{- end }}

INTERLEAVED COMMANDS (apply to subsequent files):
{{- range .Interleaved }}
   --{{ .Name | printf "%-16s" }}{{ if .Short }}-{{ .Short | printf "%-4s" }}{{ else }}      {{ end }} {{ .Usage }}
{{- end }}

EXAMPLE:
   lx -n5 file1.txt -s "Section 2" -n2 file2.txt
   (Prints 5 lines of file1, a section header, then 2 lines of file2)
`

func printHelp() {
	var globals, interleaved []CommandDef

	for _, d := range definitions {
		if d.Type == CmdGlobal {
			globals = append(globals, d)
		} else {
			interleaved = append(interleaved, d)
		}
	}

	data := struct {
		Globals     []CommandDef
		Interleaved []CommandDef
	}{globals, interleaved}

	t := template.Must(template.New("help").Parse(helpTmpl))
	if err := t.Execute(os.Stdout, data); err != nil {
		fmt.Fprintf(os.Stderr, "help template error: %v\n", err)
	}
}
