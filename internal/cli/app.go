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
		// Fallback for stdin errors (rare)
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
		_, useNull := parsed.Globals["null"]
		stdinFiles, err := readFilenamesFromStdin(useNull)
		if err != nil {
			return fmt.Errorf("read stdin: %w", err)
		}
		for _, f := range stdinFiles {
			parsed.Ops = append(parsed.Ops, Op{Action: "FILE", Value: f, Type: CmdAction})
			hasFilesOrGenerators = true
		}
	}

	// If no inputs provided via args or stdin, default to current directory "."
	if !hasFilesOrGenerators {
		parsed.Ops = append(parsed.Ops, Op{Action: "FILE", Value: ".", Type: CmdAction})
	}
	return nil
}

func processStream(parsed *ParsedArgs) error {
	var opts lx.Options
	if cfg, ok := parsed.Globals["config"]; ok {
		opts.ConfigPath = cfg
	}

	if _, ok := parsed.Globals["xml"]; ok {
		opts.OutputFormat = "xml"
	}
	if _, ok := parsed.Globals["md"]; ok {
		opts.OutputFormat = "markdown"
	}

	tmplEngine, cfg, err := opts.CompileTemplates()
	if err != nil {
		return err
	}

	cfg.ApplyGlobals(parsed.Globals)

	out, clipboardBuf, debugOut, err := determineOutput(parsed.Globals, cfg)
	if err != nil {
		return err
	}

	// 1. Initialize Logger
	cfg.Logger = lx.NewLogger(debugOut, cfg.LogLevel)

	// 2. Log loaded configurations
	for _, path := range cfg.LoadedConfigs {
		cfg.Logger.Infof("using config: %s", path)
	}

	// 3. Determine Stats Visibility
	showStats := false
	switch cfg.ShowStats {
	case "always":
		showStats = true
	case "never":
		showStats = false
	case "auto", "":
		// Default Auto behavior:
		// Show stats if output is redirected (to clipboard or file)
		_, hasCopy := parsed.Globals["copy"]
		_, hasOutput := parsed.Globals["output"]
		isClipboardMode := cfg.OutputMode == "copy"
		_, hasStdout := parsed.Globals["stdout"]

		if hasCopy || hasOutput || (isClipboardMode && !hasStdout) {
			showStats = true
		}
	}

	if f, ok := out.(*os.File); ok && f != os.Stdout {
		defer f.Close()
	}

	ops := reorderTrailingOps(parsed.Ops)
	if err := executeOps(ops, out, debugOut, opts, tmplEngine, parsed.Globals, cfg, showStats); err != nil {
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

func executeOps(ops []Op, out io.Writer, debugOut io.Writer, opts lx.Options, tmplEngine *lx.TemplateEngine, globals map[string]string, cfg *lx.Config, showStats bool) error {
	sectionCount := 0
	for _, op := range ops {
		if op.Action == "section" {
			sectionCount++
		}
	}

	// Discovery Phase
	// We run a discovery pass first to calculate totals.
	// Since filters are interleaved, we need to simulate the option state changes during discovery.
	walker := lx.NewWalker(*cfg)
	opMap := make(map[int][]lx.InputFile)
	var allFiles []lx.InputFile

	var totalSize int64
	var absFilePaths []string

	// Temporary options state for discovery
	discOpts := opts

	for i, op := range ops {
		switch op.Action {
		case "include":
			discOpts.Includes = append(discOpts.Includes, op.Value)
		case "exclude":
			discOpts.Excludes = append(discOpts.Excludes, op.Value)
		case "reset-filters":
			discOpts.Includes = nil
			discOpts.Excludes = nil
		case "FILE", "file":
			if op.Value == "-" {
				// Read stdin immediately
				data, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("read stdin: %w", err)
				}
				f := lx.StdinInputFile{Content: data}.ToInputFile()

				opMap[i] = []lx.InputFile{f}
				allFiles = append(allFiles, f)
				totalSize += f.Size
				continue
			}

			var gathered []lx.InputFile
			for f := range walker.Walk(context.TODO(), []string{op.Value}) {
				if f.LoadError != nil {
					cfg.Logger.Errorf("%v", f.LoadError)
					continue
				}

				// Apply interleaved filters
				if !lx.IsKept(f.Path, discOpts.Includes, discOpts.Excludes) {
					continue
				}

				gathered = append(gathered, f)
				allFiles = append(allFiles, f)
				absFilePaths = append(absFilePaths, f.AbsPath)
				totalSize += f.Size
			}
			opMap[i] = gathered
		}
	}

	globalCtx := lx.GlobalContext{
		TotalFiles:    len(allFiles),
		TotalSize:     totalSize,
		TokenEstimate: lx.EstimateTokens(totalSize),
		TotalSections: sectionCount + 1,
		RootPath:      findCommonRoot(absFilePaths),
		AbsRootPath:   findCommonRoot(absFilePaths),
		Args:          globals,
		Config:        *cfg,
	}

	if showStats {
		if err := tmplEngine.Stats.Execute(debugOut, lx.StatsContext{Global: globalCtx}); err != nil {
			return fmt.Errorf("stats template error: %w", err)
		}
	}

	fileIndex := 1
	prevCompact := false
	section := 1

	// Execution Phase
	for i, op := range ops {
		switch op.Action {
		case "FILE", "file":
			files := opMap[i]
			runCfg := opts.ToRunnerConfig()
			runner := lx.NewRunner(runCfg, tmplEngine, globalCtx)

			for _, f := range files {
				isCompact, err := runner.RunFile(f, fileIndex, prevCompact, section, out)
				if err != nil {
					cfg.Logger.Errorf("processing %s: %v", f.Path, err)
					continue
				}
				prevCompact = isCompact
				fileIndex++
			}

		case "section":
			runner := lx.NewRunner(opts.ToRunnerConfig(), tmplEngine, globalCtx)
			if prevCompact {
				fmt.Fprintln(out)
			}
			section++ // Increment on each new section
			if err := runner.RunSection(op.Value, section, out); err != nil {
				return err
			}
			prevCompact = false

		case "prompt":
			runner := lx.NewRunner(opts.ToRunnerConfig(), tmplEngine, globalCtx)
			if prevCompact {
				fmt.Fprintln(out)
			}
			if err := runner.RunPrompt(op.Value, section, out); err != nil {
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

		// Filter flags (update state for next iterations if we were doing deferred discovery,
		// but since we do eager discovery above, these are effectively no-ops here,
		// kept for completeness/debugging)
		case "include":
			opts.Includes = append(opts.Includes, op.Value)
		case "exclude":
			opts.Excludes = append(opts.Excludes, op.Value)
		case "reset-filters":
			opts.Includes = nil
			opts.Excludes = nil
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
		for !strings.HasPrefix(p, root+string(filepath.Separator)) && root != "." && root != "/" {
			parent := filepath.Dir(root)
			// Prevent infinite loop at system root
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
