package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/rasros/lx/pkg/lx"
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

	if err := gatherInputs(parsed); err != nil {
		if len(args) == 0 {
			printHelp()
			return nil
		}
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
	var opts lx.Options
	if cfg, ok := parsed.Globals["config"]; ok {
		opts.ConfigPath = cfg
	}

	tmplEngine, cfg, err := opts.CompileTemplates()
	if err != nil {
		return err
	}

	out, clipboardBuf, debugOut, err := determineOutput(parsed.Globals, cfg)
	if err != nil {
		return err
	}

	mode := cfg.DebugMode
	if mode == "" {
		mode = "auto"
	}

	if _, ok := parsed.Globals["quiet"]; ok {
		mode = "never"
	} else if _, ok := parsed.Globals["verbose"]; ok {
		mode = "always"
	}

	showDebug := false
	switch mode {
	case "always":
		showDebug = true
	case "never":
		showDebug = false
	case "auto":
		_, hasCopy := parsed.Globals["copy"]
		_, hasOutput := parsed.Globals["output"]
		_, hasStdout := parsed.Globals["stdout"]

		isClipboardMode := cfg.OutputMode == "copy"

		if hasCopy || hasOutput {
			showDebug = true
		} else if isClipboardMode && !hasStdout {
			showDebug = true
		} else {
			showDebug = false
		}
	}

	if f, ok := out.(*os.File); ok && f != os.Stdout {
		defer f.Close()
	}

	ops := reorderTrailingOps(parsed.Ops)
	if err := executeOps(ops, out, debugOut, showDebug, opts, tmplEngine, parsed.Globals, cfg); err != nil {
		return err
	}

	if clipboardBuf != nil {
		if err := clipboard.WriteAll(clipboardBuf.String()); err != nil {
			return fmt.Errorf("clipboard write: %w", err)
		}
	}

	return nil
}

func determineOutput(globals map[string]string, cfg *lx.Config) (io.Writer, *bytes.Buffer, io.Writer, error) {
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
		return nil, nil, nil, fmt.Errorf("flags -o/--output, -c/--copy, and -C/--stdout are mutually exclusive")
	}

	useClipboard := false
	var out io.Writer = os.Stdout
	var clipboardBuf *bytes.Buffer
	var debugOut io.Writer = os.Stderr

	if hasOutput {
		f, err := os.Create(outputPath)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("create output file: %w", err)
		}
		out = f
		debugOut = os.Stdout
	} else if hasCopy {
		useClipboard = true
		debugOut = os.Stdout
	} else if hasStdout {
		useClipboard = false
		// debugOut remains Stderr
	} else {
		if cfg.OutputMode == "copy" {
			useClipboard = true
			debugOut = os.Stdout
		}
	}

	if useClipboard {
		if clipboard.Unsupported {
			return nil, nil, nil, fmt.Errorf("clipboard support is not available on this system (install xclip or wl-copy on Linux)")
		}
		clipboardBuf = new(bytes.Buffer)
		out = clipboardBuf
	}

	return out, clipboardBuf, debugOut, nil
}

func executeOps(ops []Op, out io.Writer, debugOut io.Writer, showDebug bool, opts lx.Options, tmplEngine *lx.TemplateEngine, globals map[string]string, cfg *lx.Config) error {
	totalFiles := 0
	var totalSize int64
	sectionCount := 0
	var filePaths []string
	var absFilePaths []string

	for _, op := range ops {
		if op.Action == "FILE" || op.Action == "file" {
			totalFiles++

			if op.Value == "-" {
				continue
			}

			info, err := os.Stat(op.Value)
			if err != nil {
				return fmt.Errorf("stat %q: %w", op.Value, err)
			}
			if info.IsDir() {
				return fmt.Errorf("read %q: is a directory", op.Value)
			}

			totalSize += info.Size()
			filePaths = append(filePaths, op.Value)

			if abs, err := filepath.Abs(op.Value); err == nil {
				absFilePaths = append(absFilePaths, abs)
			} else {
				absFilePaths = append(absFilePaths, op.Value)
			}
		} else if op.Action == "section" {
			sectionCount++
		}
	}

	globalCtx := lx.GlobalContext{
		TotalFiles:    totalFiles,
		TotalSize:     totalSize,
		TotalSections: sectionCount + 1,
		RootPath:      findCommonRoot(filePaths),
		AbsRootPath:   findCommonRoot(absFilePaths),
		Args:          globals,
		Config:        *cfg,
	}

	if showDebug {
		if err := tmplEngine.Debug.Execute(debugOut, lx.DebugContext{Global: globalCtx}); err != nil {
			return fmt.Errorf("debug template error: %w", err)
		}
	}

	fileIndex := 1
	prevCompact := false

	for _, op := range ops {
		switch op.Action {
		case "FILE", "file":
			runCfg := opts.ToRunnerConfig()
			runner := lx.NewRunner(runCfg, tmplEngine, globalCtx)

			isCompact, err := runner.RunFile(op.Value, fileIndex, prevCompact, out)
			if err != nil {
				return err
			}
			prevCompact = isCompact
			fileIndex++

		case "section":
			runner := lx.NewRunner(opts.ToRunnerConfig(), tmplEngine, globalCtx)

			if prevCompact {
				fmt.Fprintln(out)
			}
			if err := runner.RunSection(op.Value, out); err != nil {
				return err
			}
			prevCompact = false

		case "prompt":
			runner := lx.NewRunner(opts.ToRunnerConfig(), tmplEngine, globalCtx)

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

func findCommonRoot(paths []string) string {
	if len(paths) == 0 {
		return "."
	}
	if len(paths) == 1 {
		return filepath.Dir(paths[0])
	}

	root := filepath.Dir(paths[0])

	for _, p := range paths[1:] {
		// While the current path doesn't start with the root, strip back the root
		for !strings.HasPrefix(p, root+string(filepath.Separator)) && root != "." && root != "/" {
			parent := filepath.Dir(root)
			if parent == root {
				return "."
			}
			root = parent
		}
		if root == "/" && !strings.HasPrefix(p, "/") {
			return "."
		}
	}
	if root == "" {
		return "."
	}
	return root
}
