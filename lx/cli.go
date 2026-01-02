package lx

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/atotto/clipboard"
)

func Run(ctx context.Context, args []string) error {
	parsed, err := Parse(args, definitions)
	if err != nil {
		return err
	}

	stopProfiling, err := setupProfiling(parsed)
	if err != nil {
		return err
	}
	defer stopProfiling()

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

	tmplEngine, cfg, err := opts.CompileTemplates()
	if err != nil {
		return err
	}

	out, clipboardBuf, err := determineOutput(parsed.Globals, cfg)
	if err != nil {
		return err
	}

	if f, ok := out.(*os.File); ok && f != os.Stdout {
		defer f.Close()
	}

	ops := reorderTrailingOps(parsed.Ops)
	if err := executeOps(ops, out, opts, tmplEngine); err != nil {
		return err
	}

	if clipboardBuf != nil {
		if err := clipboard.WriteAll(clipboardBuf.String()); err != nil {
			return fmt.Errorf("clipboard write: %w", err)
		}
	}

	return nil
}

func determineOutput(globals map[string]string, cfg *Config) (io.Writer, *bytes.Buffer, error) {
	outputPath, hasOutput := globals["output"]
	_, hasCopy := globals["copy"]
	_, hasStdout := globals["stdout"]

	flagsSet := 0
	if hasOutput {
		flagsSet++
	}
	if hasCopy {
		flagsSet++
	}
	if hasStdout {
		flagsSet++
	}
	if flagsSet > 1 {
		return nil, nil, fmt.Errorf("flags -o/--output, -c/--copy, and -C/--stdout are mutually exclusive")
	}

	useClipboard := false
	var out io.Writer = os.Stdout
	var clipboardBuf *bytes.Buffer

	if hasOutput {
		f, err := os.Create(outputPath)
		if err != nil {
			return nil, nil, fmt.Errorf("create output file: %w", err)
		}
		out = f
	} else if hasCopy {
		useClipboard = true
	} else if hasStdout {
		useClipboard = false
	} else {
		if cfg.OutputMode == "copy" {
			useClipboard = true
		}
	}

	if useClipboard {
		if clipboard.Unsupported {
			return nil, nil, fmt.Errorf("clipboard support is not available on this system (install xclip or wl-copy on Linux)")
		}
		clipboardBuf = new(bytes.Buffer)
		out = clipboardBuf
	}

	return out, clipboardBuf, nil
}

func executeOps(ops []Op, out io.Writer, opts Options, tmplEngine *TemplateEngine) error {
	for _, op := range ops {
		if op.Action == "FILE" || op.Action == "file" {
			if _, err := os.Stat(op.Value); err != nil {
				return fmt.Errorf("stat %q: %w", op.Value, err)
			}
		}
	}

	totalFiles := 0
	for _, op := range ops {
		if op.Action == "FILE" || op.Action == "file" {
			totalFiles++
		}
	}

	fileIndex := 1
	prevCompact := false

	for _, op := range ops {
		switch op.Action {
		case "FILE", "file":
			runCfg := opts.ToRunnerConfig()
			runner := NewRunner(runCfg, tmplEngine)

			isCompact, err := runner.RunFile(op.Value, fileIndex, totalFiles, prevCompact, out)
			if err != nil {
				return err
			}
			prevCompact = isCompact
			fileIndex++

		case "section":
			runner := NewRunner(opts.ToRunnerConfig(), tmplEngine)

			if prevCompact {
				fmt.Fprintln(out)
			}
			if err := runner.RunSection(op.Value, out); err != nil {
				return err
			}
			prevCompact = false

		case "prompt":
			runner := NewRunner(opts.ToRunnerConfig(), tmplEngine)

			if prevCompact {
				fmt.Fprintln(out)
			}
			if err := runner.RunPrompt(op.Value, out); err != nil {
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

	if prevCompact {
		fmt.Fprintln(out)
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

	firstActionIdx := lastActionIdx
	for i := lastActionIdx - 1; i >= 0; i-- {
		if ops[i].Type == CmdAction {
			firstActionIdx = i
		} else {
			break
		}
	}

	newOps := make([]Op, 0, len(ops))
	newOps = append(newOps, ops[:firstActionIdx]...)
	newOps = append(newOps, modifiers...)
	newOps = append(newOps, ops[firstActionIdx:lastActionIdx+1]...)
	newOps = append(newOps, others...)

	return newOps
}
