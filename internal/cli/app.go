package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
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

	isPipe := false
	if !hasFilesOrGenerators {
		_, useNull := parsed.Globals["null"]
		var stdinFiles []string
		var err error
		stdinFiles, isPipe, err = readFilenamesFromStdin(useNull)
		if err != nil {
			return fmt.Errorf("read stdin: %w", err)
		}
		for _, f := range stdinFiles {
			parsed.Ops = append(parsed.Ops, Op{Action: "FILE", Value: f, Type: CmdAction})
			hasFilesOrGenerators = true
		}
	}

	if !hasFilesOrGenerators && !isPipe {
		parsed.Ops = append(parsed.Ops, Op{Action: "FILE", Value: ".", Type: CmdAction})
	}
	return nil
}

func processStream(parsed *ParsedArgs) error {
	var opts lx.Options
	var configPath string

	if cfg, ok := parsed.Globals["config"]; ok {
		configPath = cfg
	}

	if _, ok := parsed.Globals["xml"]; ok {
		opts.OutputFormat = "xml"
	}
	if _, ok := parsed.Globals["md"]; ok {
		opts.OutputFormat = "markdown"
	}
	if _, ok := parsed.Globals["html"]; ok {
		opts.OutputFormat = "html"
	}

	cfg, err := LoadConfigChain(configPath)
	if err != nil {
		return err
	}

	lx.ApplyOptions(cfg, opts)
	applyGlobalsToConfig(cfg, parsed.Globals)

	level := determineLogLevel(parsed.Globals, cfg.Verbosity)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey && level > slog.LevelDebug {
				return slog.Attr{}
			}
			return a
		},
	}))

	session, err := lx.NewSession(cfg)
	if err != nil {
		return err
	}

	out, clipboardBuf, debugOut, err := determineOutput(parsed.Globals, cfg)
	if err != nil {
		return err
	}

	showStats := false
	switch cfg.ShowStats {
	case "always":
		showStats = true
	case "never":
		showStats = false
	case "auto", "":
		_, hasCopy := parsed.Globals["copy"]
		_, hasOutput := parsed.Globals["output"]
		isClipboardMode := cfg.OutputMode == "copy"
		_, hasStdout := parsed.Globals["stdout"]

		stdoutIsTerm := false
		if stat, err := os.Stdout.Stat(); err == nil {
			stdoutIsTerm = (stat.Mode() & os.ModeCharDevice) != 0
		}

		if hasCopy || hasOutput || (isClipboardMode && !hasStdout) || !stdoutIsTerm {
			showStats = true
		}
	}

	if level > slog.LevelError && cfg.ShowStats != "always" {
		showStats = false
	}

	if f, ok := out.(*os.File); ok && f != os.Stdout {
		defer f.Close()
	}

	ops := reorderTrailingOps(parsed.Ops)
	if err := executeOps(ops, out, debugOut, opts, session, parsed.Globals, showStats, logger, cfg); err != nil {
		return err
	}

	if clipboardBuf != nil {
		if err := clipboard.WriteAll(clipboardBuf.String()); err != nil {
			return fmt.Errorf("clipboard write: %w", err)
		}
	}

	return nil
}

func executeOps(ops []Op, out io.Writer, debugOut io.Writer, opts lx.Options, session *lx.Session, globals map[string]string, showStats bool, logger *slog.Logger, cfg *lx.Config) error {
	// 1. DISCOVERY PHASE
	walkerOpts := lx.WalkerOptions{
		FollowSymlinks: cfg.FollowSymlinks,
		ShowHidden:     cfg.ShowHidden,
		IgnoreEnabled:  cfg.IgnoreEnabled(),
		GlobalIgnore:   cfg.GlobalIgnore,
		OnIgnore: func(path string, reason string) {
			logger.Debug("ignored path", "path", path, "reason", reason)
		},
	}
	walker := lx.NewWalker(walkerOpts)

	var allFiles []lx.InputFile
	opFiles := make(map[int][]lx.InputFile)

	activeDiscoveryOpts := opts
	sectionCount := 0

	for i, op := range ops {
		switch op.Action {
		case "include":
			activeDiscoveryOpts.Includes = append(activeDiscoveryOpts.Includes, op.Value)
		case "exclude":
			activeDiscoveryOpts.Excludes = append(activeDiscoveryOpts.Excludes, op.Value)
		case "reset-filters":
			activeDiscoveryOpts.Includes, activeDiscoveryOpts.Excludes = nil, nil
		case "section":
			sectionCount++
		case "FILE", "file":
			if op.Value == "-" {
				data, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("read stdin: %w", err)
				}
				f := lx.NewBufferInputFile("stdin", data)
				opFiles[i] = []lx.InputFile{f}
				allFiles = append(allFiles, f)
				continue
			}

			var gathered []lx.InputFile
			for f := range walker.Walk(context.TODO(), []string{op.Value}) {
				if f.LoadError != nil {
					logger.Error("load error", "path", f.Path, "error", f.LoadError)
					continue
				}
				if lx.IsKept(f.Path, activeDiscoveryOpts.Includes, activeDiscoveryOpts.Excludes) {
					gathered = append(gathered, f)
					allFiles = append(allFiles, f)
				}
			}
			opFiles[i] = gathered
		}
	}

	cwd, _ := os.Getwd()
	globalCtx := lx.CreateGlobalContext(allFiles, sectionCount+1, cwd, globals)

	// CLI-specific heuristic for token estimation
	globalCtx.TokenEstimate = globalCtx.TotalSize / 4

	if showStats {
		_ = session.Engine.Stats.Execute(debugOut, lx.StatsContext{Global: globalCtx})
	}

	if err := session.Engine.Header.Execute(out, lx.HeaderContext{Global: globalCtx}); err != nil {
		return fmt.Errorf("header template error: %w", err)
	}

	// 2. EXECUTION PHASE
	fileIndex := 1
	sectionIndex := 1
	prevCompact := false
	currentActiveOpts := opts

	for i, op := range ops {
		runner := session.NewRunner(currentActiveOpts.ToRunnerConfig(), globalCtx)
		var item lx.RenderedItem
		var err error

		switch op.Action {
		case "FILE", "file":
			for _, f := range opFiles[i] {
				item, err = runner.RunFile(f, fileIndex, sectionIndex)
				if err != nil {
					logger.Error("processing failed", "path", f.Path, "error", err)
					continue
				}

				if prevCompact && !item.IsCompactView {
					fmt.Fprintln(out)
				}
				fmt.Fprint(out, item.Body)

				prevCompact = item.IsCompactView
				fileIndex++
			}
			continue

		case "section":
			if prevCompact {
				fmt.Fprintln(out)
			}
			sectionIndex++
			item, err = runner.RunSection(op.Value, sectionIndex)
			prevCompact = false

		case "prompt":
			if prevCompact {
				fmt.Fprintln(out)
			}
			item, err = runner.RunPrompt(op.Value, sectionIndex)
			prevCompact = false

		case "line-numbers":
			currentActiveOpts.LineNumbers = true
		case "no-line-numbers":
			currentActiveOpts.LineNumbers = false
		case "head":
			val, _ := strconv.Atoi(op.Value)
			currentActiveOpts.Head = &val
			currentActiveOpts.Tail, currentActiveOpts.NBoth = nil, nil
		case "tail":
			val, _ := strconv.Atoi(op.Value)
			currentActiveOpts.Tail = &val
			currentActiveOpts.Head, currentActiveOpts.NBoth = nil, nil
		case "lines":
			val, _ := strconv.Atoi(op.Value)
			currentActiveOpts.NBoth = &val
			currentActiveOpts.Head, currentActiveOpts.Tail = nil, nil
		case "reset-lines":
			currentActiveOpts.Head, currentActiveOpts.Tail, currentActiveOpts.NBoth = nil, nil, nil
		case "include":
			currentActiveOpts.Includes = append(currentActiveOpts.Includes, op.Value)
		case "exclude":
			currentActiveOpts.Excludes = append(currentActiveOpts.Excludes, op.Value)
		case "reset-filters":
			currentActiveOpts.Includes, currentActiveOpts.Excludes = nil, nil
		}

		if err != nil {
			return err
		}
		if item.Body != "" {
			fmt.Fprint(out, item.Body)
		}
	}

	if prevCompact {
		fmt.Fprintln(out)
	}

	return session.Engine.Footer.Execute(out, lx.FooterContext{Global: globalCtx})
}

func applyGlobalsToConfig(c *lx.Config, globals map[string]string) {
	if _, ok := globals["follow"]; ok {
		c.FollowSymlinks = true
	} else if _, ok := globals["no-follow"]; ok {
		c.FollowSymlinks = false
	}

	if _, ok := globals["hidden"]; ok {
		c.ShowHidden = true
	} else if _, ok := globals["no-hidden"]; ok {
		c.ShowHidden = false
	}

	if _, ok := globals["ignore"]; ok {
		t := true
		c.Ignore = &t
	} else if _, ok := globals["no-ignore"]; ok {
		f := false
		c.Ignore = &f
	}

	if _, ok := globals["stats"]; ok {
		c.ShowStats = "always"
	} else if _, ok := globals["no-stats"]; ok {
		c.ShowStats = "never"
	}
}

func determineLogLevel(globals map[string]string, configVerbosity string) slog.Level {
	if _, ok := globals["quiet"]; ok {
		return slog.LevelError + 1
	}
	if v, ok := globals["verbose"]; ok {
		return parseLevelString(v)
	}
	if v, ok := globals["verbosity"]; ok {
		count, _ := strconv.Atoi(v)
		if count >= 3 {
			return slog.LevelDebug - 1
		} else if count == 2 {
			return slog.LevelDebug
		} else if count == 1 {
			return slog.LevelInfo
		}
	}
	if configVerbosity != "" {
		return parseLevelString(configVerbosity)
	}
	return slog.LevelWarn
}

func parseLevelString(s string) slog.Level {
	switch strings.ToLower(s) {
	case "trace":
		return slog.LevelDebug - 1
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	case "silent", "quiet", "off":
		return slog.LevelError + 1
	default:
		return slog.LevelInfo
	}
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
	} else if cfg.OutputMode == "copy" {
		useClipboard = true
		debugOut = os.Stdout
	}

	if useClipboard {
		if clipboard.Unsupported {
			return nil, nil, nil, fmt.Errorf("clipboard support is not available on this system")
		}
		clipboardBuf = new(bytes.Buffer)
		out = clipboardBuf
	}

	return out, clipboardBuf, debugOut, nil
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
