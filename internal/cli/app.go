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

	// If no inputs provided via args, AND no pipe was detected, default to "."
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

	// 1. Load Configuration (Default -> Env -> CLI File)
	cfg, err := LoadConfigChain(configPath)
	if err != nil {
		return err
	}

	// 2. Apply CLI Flag overrides (verbosity, etc)
	lx.ApplyOptions(cfg, opts)
	applyGlobalsToConfig(cfg, parsed.Globals)

	// Configure Logger
	level := determineLogLevel(parsed.Globals, cfg.Verbosity)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Remove time for cleaner CLI output unless very verbose (debug/trace)
			if a.Key == slog.TimeKey && level > slog.LevelDebug {
				return slog.Attr{}
			}
			return a
		},
	}))

	// 3. Initialize Session
	session, err := lx.NewSession(cfg, logger)
	if err != nil {
		return err
	}

	out, clipboardBuf, debugOut, err := determineOutput(parsed.Globals, cfg)
	if err != nil {
		return err
	}

	logger.Debug("started", "version", Version)
	logger.Log(context.Background(), slog.LevelDebug-1, "details", "format", cfg.OutputFormat, "mode", cfg.OutputMode)

	for _, path := range cfg.LoadedConfigs {
		logger.Info("loaded config", "path", path)
	}

	if cfg.IgnoreEnabled() {
		logger.Debug("ignore logic enabled")
	} else {
		logger.Warn("ignore logic disabled (hidden and gitignored files will be shown)")
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
		logger.Info("writing output to file", "path", f.Name())
		defer f.Close()
	} else if clipboardBuf != nil {
		logger.Info("writing output to clipboard")
	}

	ops := reorderTrailingOps(parsed.Ops)
	if err := executeOps(ops, out, debugOut, opts, session, parsed.Globals, showStats); err != nil {
		return err
	}

	if clipboardBuf != nil {
		if err := clipboard.WriteAll(clipboardBuf.String()); err != nil {
			return fmt.Errorf("clipboard write: %w", err)
		}
		logger.Info("copied to clipboard", "bytes", clipboardBuf.Len())
	}

	return nil
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
		return slog.LevelError + 1 // Silent
	}
	if v, ok := globals["verbose"]; ok {
		return parseLevelString(v)
	}
	if v, ok := globals["verbosity"]; ok {
		count, _ := strconv.Atoi(v)
		if count >= 3 {
			return slog.LevelDebug - 1 // Trace
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

func executeOps(ops []Op, out io.Writer, debugOut io.Writer, opts lx.Options, session *lx.Session, globals map[string]string, showStats bool) error {
	sectionCount := 0
	for _, op := range ops {
		if op.Action == "section" {
			sectionCount++
		}
	}

	session.Logger.Debug("starting discovery phase", "operations", len(ops))
	walker := session.NewWalker()
	opMap := make(map[int][]lx.InputFile)
	var allFiles []lx.InputFile

	// Temporary options state for discovery
	discOpts := opts

	for i, op := range ops {
		// Trace
		session.Logger.Log(context.Background(), slog.LevelDebug-1, "parsing op", "index", i, "action", op.Action, "value", op.Value)

		switch op.Action {
		case "include":
			discOpts.Includes = append(discOpts.Includes, op.Value)
			session.Logger.Debug("filter added", "type", "include", "pattern", op.Value)
		case "exclude":
			discOpts.Excludes = append(discOpts.Excludes, op.Value)
			session.Logger.Debug("filter added", "type", "exclude", "pattern", op.Value)
		case "reset-filters":
			discOpts.Includes = nil
			discOpts.Excludes = nil
			session.Logger.Debug("filters reset")
		case "FILE", "file":
			if op.Value == "-" {
				session.Logger.Debug("reading from stdin")
				data, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("read stdin: %w", err)
				}
				f := lx.NewBufferInputFile("stdin", data)

				opMap[i] = []lx.InputFile{f}
				allFiles = append(allFiles, f)
				continue
			}

			var gathered []lx.InputFile
			session.Logger.Log(context.Background(), slog.LevelDebug-1, "walking target", "target", op.Value)

			for f := range walker.Walk(context.TODO(), []string{op.Value}) {
				if f.LoadError != nil {
					session.Logger.Error("load error", "error", f.LoadError)
					continue
				}

				// Apply interleaved filters
				if !lx.IsKept(f.Path, discOpts.Includes, discOpts.Excludes) {
					session.Logger.Log(context.Background(), slog.LevelDebug-1, "filtered out by interleaved rules", "path", f.Path)
					continue
				}

				gathered = append(gathered, f)
				allFiles = append(allFiles, f)
			}
			opMap[i] = gathered
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
		session.Logger.Warn("failed to get current working directory", "error", err)
	}

	globalCtx := session.CalculateGlobalContext(allFiles, sectionCount+1, cwd, globals)

	session.Logger.Info("discovery complete", "files", globalCtx.TotalFiles, "size", lx.Humanize(globalCtx.TotalSize))

	if showStats {
		if err := session.Engine.Stats.Execute(debugOut, lx.StatsContext{Global: globalCtx}); err != nil {
			return fmt.Errorf("stats template error: %w", err)
		}
	}

	if err := session.Engine.Header.Execute(out, lx.HeaderContext{Global: globalCtx}); err != nil {
		return fmt.Errorf("header template error: %w", err)
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
			runner := session.NewRunner(runCfg, globalCtx)

			for _, f := range files {
				isCompact, err := runner.RunFile(f, fileIndex, prevCompact, section, out)
				if err != nil {
					session.Logger.Error("processing failed", "path", f.Path, "error", err)
					continue
				}
				prevCompact = isCompact
				fileIndex++
			}

		case "section":
			runner := session.NewRunner(opts.ToRunnerConfig(), globalCtx)
			if prevCompact {
				fmt.Fprintln(out)
			}
			section++
			session.Logger.Debug("rendering section", "title", op.Value)
			if err := runner.RunSection(op.Value, section, out); err != nil {
				return err
			}
			prevCompact = false

		case "prompt":
			runner := session.NewRunner(opts.ToRunnerConfig(), globalCtx)
			if prevCompact {
				fmt.Fprintln(out)
			}
			session.Logger.Debug("rendering prompt")
			if err := runner.RunPrompt(op.Value, section, out); err != nil {
				return err
			}
			prevCompact = false

		case "line-numbers":
			opts.LineNumbers = true
			session.Logger.Log(context.Background(), slog.LevelDebug-1, "option set", "key", "line-numbers", "val", true)
		case "no-line-numbers":
			opts.LineNumbers = false
			session.Logger.Log(context.Background(), slog.LevelDebug-1, "option set", "key", "line-numbers", "val", false)
		case "head":
			val, _ := strconv.Atoi(op.Value)
			opts.Head = &val
			opts.Tail = nil
			opts.NBoth = nil
			session.Logger.Log(context.Background(), slog.LevelDebug-1, "option set", "key", "head", "val", val)
		case "tail":
			val, _ := strconv.Atoi(op.Value)
			opts.Tail = &val
			opts.Head = nil
			opts.NBoth = nil
			session.Logger.Log(context.Background(), slog.LevelDebug-1, "option set", "key", "tail", "val", val)
		case "lines":
			val, _ := strconv.Atoi(op.Value)
			opts.NBoth = &val
			opts.Head = nil
			opts.Tail = nil
			session.Logger.Log(context.Background(), slog.LevelDebug-1, "option set", "key", "lines", "val", val)
		case "reset-lines":
			opts.Head = nil
			opts.Tail = nil
			opts.NBoth = nil
			session.Logger.Log(context.Background(), slog.LevelDebug-1, "option set", "key", "reset-lines")

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

	if err := session.Engine.Footer.Execute(out, lx.FooterContext{Global: globalCtx}); err != nil {
		return fmt.Errorf("footer template error: %w", err)
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
