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
	// Global Config
	{Name: "config", Short: "y", Type: CmdGlobal, ValueType: ValueAny, Usage: "path to yaml config file"},
	{Name: "version", Short: "V", Type: CmdGlobal, ValueType: ValueNone, Usage: "print the version"},
	{Name: "help", Short: "h", Type: CmdGlobal, ValueType: ValueNone, Usage: "show help"},

	// Interleaved Options
	{Name: "line-numbers", Short: "l", Type: CmdInterleaved, ValueType: ValueNone, Usage: "print line numbers"},
	{Name: "no-line-numbers", Short: "L", Type: CmdInterleaved, ValueType: ValueNone, Usage: "don't print line numbers"},
	{Name: "head", Type: CmdInterleaved, ValueType: ValueNumber, Usage: "print first N lines (0 = compact/skip)"},
	{Name: "tail", Type: CmdInterleaved, ValueType: ValueNumber, Usage: "print last N lines (0 = compact/skip)"},
	{Name: "lines", Short: "n", Type: CmdInterleaved, ValueType: ValueNumber, Usage: "print N lines split between head and tail"},
	{Name: "reset-lines", Short: "N", Type: CmdInterleaved, ValueType: ValueNone, Usage: "reset lines limits (and head/tail)"},

	// Actions
	{Name: "file", Short: "f", Type: CmdAction, ValueType: ValueAny, Usage: "explicit file path"},
	{Name: "section", Short: "s", Type: CmdAction, ValueType: ValueAny, Usage: "print a section header"},
	{Name: "prompt", Short: "p", Type: CmdAction, ValueType: ValueAny, Usage: "print custom text directly"},
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
		if op.Action == "FILE" || op.Action == "file" || op.Action == "section" || op.Action == "prompt" {
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
			parsed.Ops = append(parsed.Ops, Op{Action: "FILE", Value: f, Type: CmdAction})
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

	ops := reorderTrailingOps(parsed.Ops)

	// --- Pass 1: Validate all inputs ---
	for _, op := range ops {
		if op.Action == "FILE" || op.Action == "file" {
			if _, err := os.Stat(op.Value); err != nil {
				return fmt.Errorf("stat %q: %w", op.Value, err)
			}
		}
	}

	// --- Pass 2: Compile Templates Once ---
	tmplEngine, err := opts.CompileTemplates()
	if err != nil {
		return err
	}

	// --- Pass 3: Execute and Print ---
	totalFiles := 0
	for _, op := range ops {
		if op.Action == "FILE" || op.Action == "file" {
			totalFiles++
		}
	}

	fileIndex := 1
	prevCompact := false // Track if previous output was a compact line

	for _, op := range ops {
		switch op.Action {
		case "FILE", "file":
			runCfg := opts.ToRunnerConfig()
			runner := NewRunner(runCfg, tmplEngine)

			isCompact, err := runner.RunFile(op.Value, fileIndex, totalFiles, prevCompact, os.Stdout)
			if err != nil {
				return err
			}
			prevCompact = isCompact
			fileIndex++

		case "section":
			runner := NewRunner(opts.ToRunnerConfig(), tmplEngine)

			if prevCompact {
				fmt.Fprintln(os.Stdout)
			}
			if err := runner.RunSection(op.Value, os.Stdout); err != nil {
				return err
			}
			prevCompact = false

		case "prompt":
			runner := NewRunner(opts.ToRunnerConfig(), tmplEngine)

			if prevCompact {
				fmt.Fprintln(os.Stdout)
			}
			if err := runner.RunPrompt(op.Value, os.Stdout); err != nil {
				return err
			}
			prevCompact = false

		case "line-numbers":
			opts.LineNumbers = true
		case "no-line-numbers":
			opts.LineNumbers = false
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
		case "lines":
			val, _ := strconv.Atoi(op.Value)
			opts.NBoth = val
			opts.NSet = true
			opts.HeadSet = false
			opts.TailSet = false
		case "reset-lines":
			opts.Head, opts.HeadSet = 0, false
			opts.Tail, opts.TailSet = 0, false
			opts.NBoth, opts.NSet = 0, false
		}
	}

	// If the stream ends with a compact file (which only prints \n),
	// add one more newline to visually separate it from the shell prompt.
	if prevCompact {
		fmt.Fprintln(os.Stdout)
	}

	return nil
}

func reorderTrailingOps(ops []Op) []Op {
	lastActionIdx := -1
	for i := len(ops) - 1; i >= 0; i-- {
		if ops[i].Type == CmdAction {
			lastActionIdx = i
			break
		}
	}

	if lastActionIdx == -1 || lastActionIdx == len(ops)-1 {
		return ops
	}

	modifiers := make([]Op, 0)
	others := make([]Op, 0)

	for _, op := range ops[lastActionIdx+1:] {
		if op.Type == CmdInterleaved {
			modifiers = append(modifiers, op)
		} else {
			others = append(others, op)
		}
	}

	if len(modifiers) == 0 {
		return ops
	}

	newOps := make([]Op, 0, len(ops))
	newOps = append(newOps, ops[:lastActionIdx]...)
	newOps = append(newOps, modifiers...)
	newOps = append(newOps, ops[lastActionIdx])
	newOps = append(newOps, others...)

	return newOps
}

const helpTmpl = `NAME:
   lx - print files with headers, slicing, and go-templates

USAGE:
   lx [global options] [command state options] [files/actions...]

GLOBAL OPTIONS:
{{- range .Globals }}
   --{{ .Name | printf "%-16s" }}{{ if .Short }}-{{ .Short | printf "%-4s" }}{{ else }}      {{ end }} {{ .Usage }}
{{- end }}

INTERLEAVED OPTIONS (apply to subsequent files):
{{- range .Interleaved }}
   --{{ .Name | printf "%-16s" }}{{ if .Short }}-{{ .Short | printf "%-4s" }}{{ else }}      {{ end }} {{ .Usage }}
{{- end }}

ACTIONS (printed in order):
{{- range .Actions }}
   --{{ .Name | printf "%-16s" }}{{ if .Short }}-{{ .Short | printf "%-4s" }}{{ else }}      {{ end }} {{ .Usage }}
{{- end }}

EXAMPLE:
   lx -n5 file1.txt -s "Section 2" -n2 file2.txt
   (Prints 5 lines of file1, a section header, then 2 lines of file2)
`

func printHelp() {
	var globals, interleaved, actions []CommandDef

	for _, d := range definitions {
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
