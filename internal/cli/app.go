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
			for _, f := range stdinFiles {
				parsed.Ops = append(parsed.Ops, Op{Action: "FILE", Value: f, Type: CmdAction})
			}
			hasFilesOrGenerators = true
		}
	}

	if !hasFilesOrGenerators {
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
	// Set default logger (optional, if your pkg/lx uses it, otherwise pass it down)
	slog.SetDefault(logger)

	out, clipBuf, debugOut, err := determineOutput(parsed.Globals, cliOpts.OutputMode)
	if err != nil {
		return err
	}

	var outPath string
	if outputPath, ok := parsed.Globals["output"]; ok {
		if abs, err := filepath.Abs(outputPath); err == nil {
			outPath = abs
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

	ops := reorderTrailingOps(parsed.Ops)
	for _, op := range ops {
		switch op.Action {
		case "head":
			val, _ := strconv.Atoi(op.Value)
			runCfg.Head, runCfg.Tail = val, 0
			stream.WithRunnerConfig(runCfg)
		case "tail":
			val, _ := strconv.Atoi(op.Value)
			runCfg.Tail, runCfg.Head = val, 0
			stream.WithRunnerConfig(runCfg)
		case "lines":
			val, _ := strconv.Atoi(op.Value)
			runCfg.Head, runCfg.Tail = (val+1)/2, val/2
			stream.WithRunnerConfig(runCfg)
		case "reset-lines":
			runCfg.Head, runCfg.Tail = -1, 0
			stream.WithRunnerConfig(runCfg)
		case "line-numbers":
			runCfg.LineNumbers = true
			stream.WithRunnerConfig(runCfg)
		case "no-line-numbers":
			runCfg.LineNumbers = false
			stream.WithRunnerConfig(runCfg)
		case "include":
			includes = append(includes, op.Value)
		case "exclude":
			excludes = append(excludes, op.Value)
		case "reset-filters":
			includes = nil
			excludes = nil

		case "FILE", "file":
			if op.Value == "-" {
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

			// Determine walker settings
			// If action is "file" (specifically -f), force inclusion of hidden and ignored files
			walkerShowHidden := cfg.ShowHidden
			walkerIgnoreEnabled := cfg.IgnoreEnabled

			if op.Action == "file" {
				walkerShowHidden = true
				walkerIgnoreEnabled = false
			}

			walker := lx.NewWalker(lx.WalkerOptions{
				FS:             os.DirFS(fsRoot),
				FollowSymlinks: cfg.FollowSymlinks,
				ShowHidden:     walkerShowHidden,
				IgnoreEnabled:  walkerIgnoreEnabled,
				GlobalIgnore:   cfg.GlobalIgnore,
				Includes:       includes,
				Excludes:       excludes,
			})

			for f := range walker.Walk(ctx, []string{walkPath}) {
				if outPath != "" {
					fullAbs, _ := filepath.Abs(filepath.Join(fsRoot, f.Path))
					if fullAbs == outPath {
						continue
					}
				}
				if f.LoadError != nil {
					logger.Error("load error", "path", f.Path, "error", f.LoadError)
					continue
				}
				stream.AddFile(f)
			}
		case "section":
			stream.AddSection(op.Value)
		case "prompt":
			stream.AddPrompt(op.Value)
		}
	}

	if f, ok := out.(*os.File); ok && f != os.Stdout {
		defer f.Close()
	}

	err = stream.Execute(ctx, out)
	if err != nil {
		return err
	}

	if clipBuf != nil {
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
		c.FollowSymlinks = true
	}
	if _, ok := globals["hidden"]; ok {
		c.ShowHidden = true
	}
	if _, ok := globals["no-ignore"]; ok {
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
	} else if hasCopy || defaultMode == "copy" {
		clipBuf = new(bytes.Buffer)
		out = clipBuf
		debugOut = os.Stdout
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
