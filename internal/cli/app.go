package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/rasros/lx/pkg/lx"
)

func Run(ctx context.Context, args []string) error {
	// We can't use the configured logger yet, but we can log to default if needed.
	// However, usually we wait until flags are parsed to know the verbosity.
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
			printShortHelp()
			return nil
		}
		return err
	}

	return processStream(ctx, parsed)
}

func handleGlobals(parsed *ParsedArgs) bool {
	// Check for help Op specifically to distinguish short/long
	for _, op := range parsed.Ops {
		if op.Action == "help" {
			if op.IsShort {
				printShortHelp()
			} else {
				printLongHelp()
			}
			return true
		}
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
		stdinFiles, isPipe, err := readFilenamesFromStdin(useNull)
		if err != nil {
			return fmt.Errorf("read stdin: %w", err)
		}
		if isPipe {
			slog.Debug("Detected input from stdin pipe", "count", len(stdinFiles))
			for _, f := range stdinFiles {
				parsed.Ops = append(parsed.Ops, Op{Action: "FILE", Value: f, Type: CmdAction})
			}
			hasFilesOrGenerators = true
		}
	}

	if !hasFilesOrGenerators {
		slog.Debug("No inputs detected, defaulting to current directory '.'")
		parsed.Ops = append(parsed.Ops, Op{Action: "FILE", Value: ".", Type: CmdAction})
	}
	return nil
}

func processStream(ctx context.Context, parsed *ParsedArgs) error {
	// 1. Load Config (First pass to get settings)
	cfg, cliOpts, err := LoadConfigChain(parsed.Globals["config"])
	if err != nil {
		return err
	}

	applyGlobalsToConfig(cfg, parsed.Globals)
	if _, ok := parsed.Globals["xml"]; ok {
		cfg.OutputFormat = "xml"
	} else if _, ok := parsed.Globals["html"]; ok {
		cfg.OutputFormat = "html"
	}

	// 2. Setup Logger
	level := determineLogLevel(parsed, cliOpts.Verbosity)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey && level > slog.LevelDebug {
				return slog.Attr{}
			}
			return a
		},
	}))
	slog.SetDefault(logger)

	slog.Debug("Logger initialized", "level", level.String())
	slog.Debug("Configuration loaded", "format", cfg.OutputFormat, "ignore_enabled", cfg.IgnoreEnabled)

	// 3. Determine Outputs
	out, clipBuf, debugOut, err := determineOutput(parsed.Globals, cliOpts.OutputMode)
	if err != nil {
		return err
	}

	var outPath string
	if outputPath, ok := parsed.Globals["output"]; ok {
		if abs, err := filepath.Abs(outputPath); err == nil {
			outPath = abs
			slog.Debug("Output file path resolved", "path", outPath)
		}
	}

	runCfg := lx.RunnerConfig{
		Head:        -1,
		Tail:        0,
		LineNumbers: false,
	}

	stream, err := lx.NewStream(cfg, runCfg)
	if err != nil {
		return err
	}

	var includes, excludes []string

	// 4. Process Operations
	ops := reorderTrailingOps(parsed.Ops)
	slog.Debug("Processing operations", "count", len(ops))

	for _, op := range ops {
		switch op.Action {
		case "head":
			val, _ := strconv.Atoi(op.Value)
			slog.Debug("Setting head limit", "value", val)
			runCfg.Head, runCfg.Tail = val, 0
			stream.WithRunnerConfig(runCfg)
		case "tail":
			val, _ := strconv.Atoi(op.Value)
			slog.Debug("Setting tail limit", "value", val)
			runCfg.Tail, runCfg.Head = val, 0
			stream.WithRunnerConfig(runCfg)
		case "lines":
			val, _ := strconv.Atoi(op.Value)
			slog.Debug("Setting lines limit", "value", val)
			runCfg.Head, runCfg.Tail = (val+1)/2, val/2
			stream.WithRunnerConfig(runCfg)
		case "reset-lines":
			slog.Debug("Resetting line limits")
			runCfg.Head, runCfg.Tail = -1, 0
			stream.WithRunnerConfig(runCfg)
		case "line-numbers":
			slog.Debug("Enabling line numbers")
			runCfg.LineNumbers = true
			stream.WithRunnerConfig(runCfg)
		case "no-line-numbers":
			slog.Debug("Disabling line numbers")
			runCfg.LineNumbers = false
			stream.WithRunnerConfig(runCfg)
		case "include":
			slog.Debug("Adding include filter", "pattern", op.Value)
			includes = append(includes, op.Value)
		case "exclude":
			slog.Debug("Adding exclude filter", "pattern", op.Value)
			excludes = append(excludes, op.Value)
		case "reset-filters":
			slog.Debug("Resetting filters")
			includes = nil
			excludes = nil

		case "FILE", "file":
			if op.Value == "-" {
				slog.Info("Reading content from stdin")
				data, _ := io.ReadAll(os.Stdin)
				stream.AddFile(lx.NewBufferInputFile("stdin", data))
				continue
			}

			rootPath := op.Value
			var fsRoot, walkPath string

			if filepath.IsAbs(rootPath) {
				fsRoot = filepath.Dir(rootPath)
				walkPath = filepath.Base(rootPath)
			} else {
				clean := filepath.Clean(rootPath)
				if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
					abs, _ := filepath.Abs(clean)
					fsRoot = abs
					walkPath = "."
				} else {
					fsRoot = "."
					walkPath = clean
				}
			}

			walkPath = filepath.ToSlash(walkPath)

			walkerShowHidden := cfg.ShowHidden
			walkerIgnoreEnabled := cfg.IgnoreEnabled

			if op.Action == "file" {
				slog.Debug("Forcing single file inclusion (ignoring ignore rules)", "path", rootPath)
				walkerShowHidden = true
				walkerIgnoreEnabled = false
			}

			slog.Debug("Starting walker",
				"fs_root", fsRoot,
				"walk_path", walkPath,
				"includes", includes,
				"excludes", excludes,
				"ignore", walkerIgnoreEnabled,
			)

			walker := lx.NewWalker(lx.WalkerOptions{
				FS:             os.DirFS(fsRoot),
				FollowSymlinks: cfg.FollowSymlinks,
				ShowHidden:     walkerShowHidden,
				IgnoreEnabled:  walkerIgnoreEnabled,
				GlobalIgnore:   cfg.GlobalIgnore,
				Includes:       includes,
				Excludes:       excludes,
			})

			count := 0
			for f := range walker.Walk(ctx, []string{walkPath}) {
				if outPath != "" {
					fullAbs, _ := filepath.Abs(filepath.Join(fsRoot, f.Path))
					if fullAbs == outPath {
						slog.Debug("Skipping output file to avoid recursion", "path", f.Path)
						continue
					}
				}
				if f.LoadError != nil {
					logger.Error("Failed to load file", "path", f.Path, "error", f.LoadError)
					continue
				}
				slog.Debug("Adding file to stream", "path", f.Path, "size", f.Size)
				stream.AddFile(f)
				count++
			}
			slog.Debug("Walker finished", "path", rootPath, "files_found", count)

		case "section":
			slog.Debug("Adding section", "title", op.Value)
			stream.AddSection(op.Value)
		case "prompt":
			slog.Debug("Adding prompt", "length", len(op.Value))
			stream.AddPrompt(op.Value)
		}
	}

	if f, ok := out.(*os.File); ok && f != os.Stdout {
		slog.Info("Writing output to file", "path", f.Name())
		defer f.Close()
	}

	slog.Info("Executing pipeline")
	err = stream.Execute(ctx, out)
	if err != nil {
		slog.Error("Pipeline execution failed", "error", err)
		return err
	}

	if clipBuf != nil {
		slog.Info("Copying output to clipboard", "bytes", clipBuf.Len())
		if err := clipboard.WriteAll(clipBuf.String()); err != nil {
			return fmt.Errorf("clipboard write: %w", err)
		}
	}

	handleStatsDisplay(parsed, cliOpts, stream, debugOut)
	return nil
}

func handleStatsDisplay(parsed *ParsedArgs, cliOpts *CliConfig, stream *lx.Stream, debugOut io.Writer) {
	showStatsFlag := cliOpts.ShowStats
	if _, ok := parsed.Globals["stats"]; ok {
		showStatsFlag = "always"
	} else if _, ok := parsed.Globals["no-stats"]; ok {
		showStatsFlag = "never"
	}

	if showStatsFlag == "never" {
		return
	}

	show := (showStatsFlag == "always")
	if !show {
		_, hasCopy := parsed.Globals["copy"]
		_, hasOutput := parsed.Globals["output"]
		stdoutIsTerm := true
		if stat, err := os.Stdout.Stat(); err == nil {
			stdoutIsTerm = (stat.Mode() & os.ModeCharDevice) != 0
		}
		if hasCopy || hasOutput || !stdoutIsTerm {
			show = true
		}
	}

	if show {
		_ = stream.GetEngine().Stats.Execute(debugOut, lx.StatsContext{
			Global: stream.GetGlobalContext(),
		})
	}
}

func applyGlobalsToConfig(c *lx.Config, globals map[string]string) {
	if _, ok := globals["follow"]; ok {
		slog.Debug("Override: Follow symlinks enabled")
		c.FollowSymlinks = true
	}
	if _, ok := globals["hidden"]; ok {
		slog.Debug("Override: Show hidden files enabled")
		c.ShowHidden = true
	}
	if _, ok := globals["no-ignore"]; ok {
		slog.Debug("Override: Ignore files disabled")
		c.IgnoreEnabled = false
	}
}

func determineLogLevel(parsed *ParsedArgs, configVerbosity string) slog.Level {
	if _, ok := parsed.Globals["quiet"]; ok {
		return slog.LevelError + 1
	}

	// Calculate level based on Ops to handle mix of -vv and --verbose=debug
	count := 0
	explicitLevel := ""

	for _, op := range parsed.Ops {
		if op.Action == "verbose" {
			if op.Value == "true" {
				count++
			} else if op.Value != "" {
				explicitLevel = op.Value
			}
		}
	}

	// 1. Explicit level wins (--verbose=trace)
	if explicitLevel != "" {
		return parseLevelString(explicitLevel)
	}

	// 2. Flags count wins (-vv)
	if count > 0 {
		if count >= 2 {
			return slog.LevelDebug
		}
		return slog.LevelInfo
	}

	// 3. Config file fallback
	return parseLevelString(configVerbosity)
}

func parseLevelString(s string) slog.Level {
	// Handle numeric string from config
	if c, err := strconv.Atoi(s); err == nil {
		if c >= 2 {
			return slog.LevelDebug
		}
		if c == 1 {
			return slog.LevelInfo
		}
		return slog.LevelWarn
	}

	switch strings.ToLower(s) {
	case "trace", "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "error":
		return slog.LevelError
	case "silent":
		return slog.LevelError + 1
	default:
		return slog.LevelWarn
	}
}

func determineOutput(globals map[string]string, defaultMode string) (io.Writer, *bytes.Buffer, io.Writer, error) {
	outputPath, hasOutput := globals["output"]
	_, hasCopy := globals["copy"]

	var out io.Writer = os.Stdout
	var clipBuf *bytes.Buffer
	var debugOut io.Writer = os.Stderr

	if hasOutput {
		f, err := os.Create(outputPath)
		if err != nil {
			return nil, nil, nil, err
		}
		out = f
		debugOut = os.Stdout
		slog.Debug("Output set to file", "path", outputPath)
	} else if hasCopy || defaultMode == "copy" {
		clipBuf = new(bytes.Buffer)
		out = clipBuf
		debugOut = os.Stdout
		slog.Debug("Output set to clipboard buffer")
	} else {
		slog.Debug("Output set to stdout")
	}

	return out, clipBuf, debugOut, nil
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
