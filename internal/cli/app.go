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
	parsed, err := Parse(args, definitions)
	if err != nil {
		return fmt.Errorf("argument parsing failed: %w", err)
	}

	stopProfiling, err := setupProfiling(parsed)
	if err != nil {
		return fmt.Errorf("profiling setup failed: %w", err)
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
		return fmt.Errorf("input gathering failed: %w", err)
	}

	return processStream(ctx, parsed)
}

func handleGlobals(parsed *ParsedArgs) bool {
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
			slog.Debug("Detected input from actions")
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
		} else {
			slog.Debug("No stdin pipe detected")
		}
	}

	if !hasFilesOrGenerators {
		slog.Debug("No inputs detected, defaulting to current directory '.'")
		parsed.Ops = append(parsed.Ops, Op{Action: "FILE", Value: ".", Type: CmdAction})
	}
	return nil
}

func processStream(ctx context.Context, parsed *ParsedArgs) error {
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

	level, err := determineLogLevel(parsed, cliOpts.Verbosity)
	if err != nil {
		return err
	}

	var handler slog.Handler

	if level > slog.LevelDebug {
		handler = NewCliHandler(os.Stderr, level)
	} else {
		handler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: level,
		})
	}

	slog.SetDefault(slog.New(handler))

	slog.Debug("Logger initialized", "level", level.String())
	slog.Debug("Configuration loaded",
		"format", cfg.OutputFormat,
		"ignore_enabled", cfg.IgnoreEnabled,
		"ignore_hidden", cfg.IgnoreHidden,
		"ignore_dir_symlinks", cfg.IgnoreDirSymlinks,
		"ignore_file_symlinks", cfg.IgnoreFileSymlinks,
	)

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

	stream.WithOnFileError(func(f lx.InputFile, err error) {
		slog.Error("Failed to read file", "path", f.Path, "error", err)
	})

	var includes, excludes []string

	ops := reorderTrailingOps(parsed.Ops)
	slog.Debug("Processing operations", "total_ops", len(ops))

	for i, op := range ops {
		slog.Debug("Processing op", "index", i, "action", op.Action, "value", op.Value)

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
		case "reset-line-numbers":
			slog.Debug("Resetting line numbers")
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
				data, err := io.ReadAll(os.Stdin)
				if err != nil {
					slog.Error("Failed to read from stdin", "error", err)
					continue
				}
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

			walkerIgnoreHidden := cfg.IgnoreHidden
			walkerIgnoreDirSymlinks := cfg.IgnoreDirSymlinks
			walkerIgnoreFileSymlinks := cfg.IgnoreFileSymlinks
			walkerIgnoreEnabled := cfg.IgnoreEnabled

			if op.Action == "file" {
				slog.Debug("Action 'file' used: Forcing inclusion (bypassing ignore/hidden rules)", "path", rootPath)
				walkerIgnoreHidden = false
				walkerIgnoreEnabled = false
				walkerIgnoreDirSymlinks = false
				walkerIgnoreFileSymlinks = false
			}

			slog.Debug("Initializing Walker",
				"fs_root", fsRoot,
				"walk_path", walkPath,
				"includes_count", len(includes),
				"excludes_count", len(excludes),
				"ignore_enabled", walkerIgnoreEnabled,
				"ignore_hidden", walkerIgnoreHidden,
				"ignore_dir_symlinks", walkerIgnoreDirSymlinks,
				"ignore_file_symlinks", walkerIgnoreFileSymlinks,
			)

			walker := lx.NewWalker(lx.WalkerOptions{
				FS:                 os.DirFS(fsRoot),
				Root:               fsRoot,
				IgnoreDirSymlinks:  walkerIgnoreDirSymlinks,
				IgnoreFileSymlinks: walkerIgnoreFileSymlinks,
				IgnoreHidden:       walkerIgnoreHidden,
				IgnoreEnabled:      walkerIgnoreEnabled,
				GlobalIgnore:       cfg.GlobalIgnore,
				Includes:           includes,
				Excludes:           excludes,
				// UPDATED: Now accepts lx.IgnoreReason (enum) instead of string
				OnIgnore: func(path string, reason lx.IgnoreReason, source string) {
					args := []any{"path", path, "reason", reason.String()}
					if source != "" {
						args = append(args, "source", source)
					}
					slog.Debug("Walker ignored path", args...)
				},
			})

			count := 0
			for f := range walker.Walk(ctx, []string{walkPath}) {
				if outPath != "" {
					fullAbs, _ := filepath.Abs(filepath.Join(fsRoot, f.Path))
					if fullAbs == outPath {
						slog.Warn("Skipping output file to avoid infinite recursion", "path", f.Path)
						continue
					}
				}
				if f.LoadError != nil {
					slog.Error("Failed to access file during walk", "path", f.Path, "error", f.LoadError)
					continue
				}

				slog.Debug("File accepted by walker", "path", f.Path, "size", f.Size)
				stream.AddFile(f)
				count++
			}
			slog.Debug("Walker finished", "root", rootPath, "files_found", count)

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

	slog.Info("Executing stream pipeline...")
	err = stream.Execute(ctx, out)
	if err != nil {
		slog.Error("Pipeline execution failed", "error", err)
		return err
	}

	if clipBuf != nil {
		slog.Info("Copying output to clipboard", "bytes", clipBuf.Len())
		if err := clipboard.WriteAll(clipBuf.String()); err != nil {
			return fmt.Errorf("clipboard write failed: %w", err)
		}
		slog.Info("Clipboard copy successful")
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
	} else if _, ok := parsed.Globals["quiet"]; ok {
		showStatsFlag = "never"
	}

	if showStatsFlag == "never" {
		return
	}

	show := showStatsFlag == "always"
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
		err := stream.GetEngine().Stats.Execute(debugOut, lx.StatsContext{
			Global: stream.GetGlobalContext(),
		})
		if err != nil {
			slog.Error("Failed to render stats", "error", err)
		}
	}
}

func applyGlobalsToConfig(c *lx.Config, globals map[string]string) {
	// Flags "Show/Follow" INVERT the internal "Ignore" settings.
	if _, ok := globals["follow"]; ok {
		slog.Debug("Override: Follow directory symlinks enabled via flag")
		c.IgnoreDirSymlinks = false
	}
	if _, ok := globals["no-follow"]; ok {
		slog.Debug("Override: Follow directory symlinks disabled via flag")
		c.IgnoreDirSymlinks = true
	}

	if _, ok := globals["no-links"]; ok {
		slog.Debug("Override: Hide file symlinks enabled via flag")
		c.IgnoreFileSymlinks = true
	}
	if _, ok := globals["links"]; ok {
		slog.Debug("Override: Show file symlinks enabled via flag")
		c.IgnoreFileSymlinks = false
	}

	if _, ok := globals["hidden"]; ok {
		slog.Debug("Override: Show hidden files enabled via flag")
		c.IgnoreHidden = false
	}
	if _, ok := globals["no-hidden"]; ok {
		slog.Debug("Override: Show hidden files disabled via flag")
		c.IgnoreHidden = true
	}

	if _, ok := globals["no-ignore"]; ok {
		slog.Debug("Override: Ignore files disabled via flag")
		c.IgnoreEnabled = false
	}
	if _, ok := globals["ignore"]; ok {
		slog.Debug("Override: Ignore files enabled via flag")
		c.IgnoreEnabled = true
	}
}

func determineLogLevel(parsed *ParsedArgs, configVerbosity string) (slog.Level, error) {
	if _, ok := parsed.Globals["quiet"]; ok {
		return slog.LevelError + 1, nil
	}

	count := 0
	var explicitLevel slog.Level
	hasExplicit := false

	for _, op := range parsed.Ops {
		if op.Action == "verbose" {
			if op.Value == "true" {
				count++
			} else if op.Value != "" {
				lvl, err := parseLogLevel(op.Value)
				if err != nil {
					return 0, fmt.Errorf("invalid verbosity level %s", op.Value)
				}
				explicitLevel = lvl
				hasExplicit = true
			}
		}
	}

	if hasExplicit {
		return explicitLevel, nil
	}

	if count > 0 {
		if count >= 2 {
			return slog.LevelDebug, nil
		}
		return slog.LevelInfo, nil
	}

	if lvl, err := parseLogLevel(configVerbosity); err == nil {
		return lvl, nil
	}

	return slog.LevelWarn, nil
}

func parseLogLevel(s string) (slog.Level, error) {
	if c, err := strconv.Atoi(s); err == nil {
		if c >= 2 {
			return slog.LevelDebug, nil
		}
		if c == 1 {
			return slog.LevelInfo, nil
		}
		return slog.LevelWarn, nil
	}

	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	case "silent":
		return slog.LevelError + 1, nil
	default:
		return 0, fmt.Errorf("unknown log level: %q", s)
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
