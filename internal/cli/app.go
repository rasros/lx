package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
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
	initialLevel, err := determineLogLevel(parsed, "warn")
	if err != nil {
		return err
	}
	setupLogger(initialLevel)
	slog.Debug("Logger initialized (early)", "level", initialLevel.String())

	cfg, cliOpts, err := LoadConfigChain(parsed.Globals["config"])
	if err != nil {
		return err
	}

	finalLevel, err := determineLogLevel(parsed, cliOpts.Verbosity)
	if err != nil {
		return err
	}

	if finalLevel != initialLevel {
		setupLogger(finalLevel)
		slog.Debug("Logger level updated from config", "new_level", finalLevel.String())
	}

	slog.Debug("Configuration loaded",
		"format", cfg.OutputFormat,
		"ignore_enabled", cfg.IgnoreEnabled,
		"ignore_hidden", cfg.IgnoreHidden,
		"ignore_dir_symlinks", cfg.IgnoreDirSymlinks,
		"ignore_file_symlinks", cfg.IgnoreFileSymlinks,
	)

	applyGlobalsToConfig(cfg, parsed.Globals)
	if _, ok := parsed.Globals["xml"]; ok {
		cfg.OutputFormat = "xml"
	} else if _, ok := parsed.Globals["html"]; ok {
		cfg.OutputFormat = "html"
	}

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

	defaultRunCfg := lx.RunnerConfig{
		Head:        -1,
		Tail:        0,
		LineNumbers: false,
	}
	runCfg := defaultRunCfg

	stream, err := lx.NewStream(cfg, runCfg)
	if err != nil {
		return err
	}

	stream.WithOnFileError(func(f lx.InputFile, err error) {
		slog.Error("Failed to read file", "path", f.Path, "error", err)
	})
	var tempCleanups []func()
	defer func() {
		for _, cleanup := range tempCleanups {
			cleanup()
		}
	}()

	var includes, excludes []string
	hasFileInGroup := false

	ops := reorderTrailingOps(parsed.Ops)
	slog.Debug("Processing operations", "total_ops", len(ops))

	globalIgnoreRules := LoadGlobalIgnorePatterns()
	treePrecomp := precomputeTreeStrings(ops, cfg, globalIgnoreRules)

	for i, op := range ops {
		slog.Debug("Processing op", "index", i, "action", op.Action, "value", op.Value)

		// Group boundary: when an interleaved flag follows file actions, reset all
		// interleaved state to defaults before applying the new flag.
		if op.Type == CmdInterleaved && hasFileInGroup {
			runCfg = defaultRunCfg
			includes = nil
			excludes = nil
			hasFileInGroup = false
			stream.WithRunnerConfig(runCfg)
			slog.Debug("Group boundary: resetting interleaved state")
		}

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
		case "line-numbers":
			slog.Debug("Enabling line numbers")
			runCfg.LineNumbers = true
			stream.WithRunnerConfig(runCfg)
		case "functions":
			slog.Debug("Enabling function skeleton")
			runCfg.SkeletonFunctions = true
			stream.WithRunnerConfig(runCfg)
		case "types":
			slog.Debug("Enabling type skeleton")
			runCfg.SkeletonTypes = true
			stream.WithRunnerConfig(runCfg)
		case "include":
			slog.Debug("Adding include filter", "pattern", op.Value)
			includes = append(includes, op.Value)
		case "exclude":
			slog.Debug("Adding exclude filter", "pattern", op.Value)
			excludes = append(excludes, op.Value)
		case "FILE", "file":
			hasFileInGroup = true
			if treePrecomp.skipFileOps[i] {
				slog.Debug("Skipping file content (tree-only mode)", "path", op.Value)
				continue
			}

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

			rawPath := op.Value
			isForced := op.Action == "file"

			if lx.IsHTTPURL(rawPath) {
				if cfg.ExpandArchives && lx.IsHTTPArchiveURL(rawPath) {
					// Expandable archives bypass the include check: the filter applies to their contents.
					if !isForced && !lx.IsKept(rawPath, nil, excludes) {
						slog.Debug("Skipping URL archive due to exclude filter", "url", rawPath)
						continue
					}

					tempPath, cleanup, err := lx.DownloadURLToTempFile(ctx, rawPath)
					if err != nil {
						slog.Error("Failed to download URL archive", "url", rawPath, "error", err)
						continue
					}
					tempCleanups = append(tempCleanups, cleanup)

					archiveWalker := newArchiveWalker(cfg, isForced)
					archiveWalker.OnIgnore = func(p, reason string) {
						slog.Debug("Ignored in URL archive", "path", rawPath+"/"+p, "reason", reason)
					}
					archiveIncludes := includes
					if isForced {
						archiveIncludes = nil
					}
					if err := lx.ExpandArchive(ctx, tempPath, rawPath, archiveWalker, archiveIncludes, outPath, stream); err != nil {
						slog.Error("Failed to expand URL archive", "url", rawPath, "error", err)
					}
					continue
				}

				if !isForced && !lx.IsKept(rawPath, includes, excludes) {
					slog.Debug("Skipping URL due to filters", "url", rawPath)
					continue
				}
				urlFile, err := lx.NewURLInputFile(rawPath)
				if err != nil {
					slog.Error("Failed to create URL input", "url", rawPath, "error", err)
					continue
				}
				slog.Debug("URL accepted", "url", urlFile.Path)
				stream.AddFile(urlFile)
				continue
			}

			var fsys fs.FS
			var walkRoot string
			var displayPrefix string

			absPath, err := filepath.Abs(rawPath)
			if err != nil {
				slog.Error("Failed to resolve absolute path", "path", rawPath, "error", err)
				continue
			}

			stat, err := os.Stat(absPath)
			if err != nil {
				// Prevent stat errors if the missing file (e.g., from git diff) is filtered out anyway
				if !isForced && !lx.IsKept(rawPath, includes, excludes) {
					slog.Debug("Skipping missing path due to filters", "path", rawPath)
					continue
				}
				slog.Error("Failed to stat path", "path", absPath, "error", err)
				continue
			}

			if !stat.IsDir() {
				// Prevent processing an explicit file argument if it matches an exclude filter.
				// Expandable archives bypass the include check: the filter applies to their contents.
				isExpandableArchive := cfg.ExpandArchives && lx.IsArchivePath(rawPath)
				if !isForced && !isExpandableArchive && !lx.IsKept(rawPath, includes, excludes) {
					slog.Debug("Skipping file due to filters", "path", rawPath)
					continue
				}
				if !isForced && isExpandableArchive && !lx.IsKept(rawPath, nil, excludes) {
					slog.Debug("Skipping archive due to exclude filter", "path", rawPath)
					continue
				}

				rawPathClean := filepath.Clean(rawPath)

				if !filepath.IsAbs(rawPathClean) && !strings.HasPrefix(rawPathClean, "..") {
					fsys = os.DirFS(".")
					walkRoot = filepath.ToSlash(rawPathClean)
				} else {
					fsys = os.DirFS(filepath.Dir(absPath))
					walkRoot = filepath.Base(absPath)
					displayPrefix = filepath.Dir(rawPathClean)
				}
			} else {
				fsys = os.DirFS(absPath)
				walkRoot = "."
				displayPrefix = filepath.Clean(rawPath)
			}

			var baseRules []string
			var overrideRules []string

			if cfg.IgnoreEnabled {
				baseRules = append(baseRules, globalIgnoreRules...)
			}

			if cfg.IgnoreHidden && !isForced {
				overrideRules = append(overrideRules, ".*")
			}

			if !isForced {
				overrideRules = append(overrideRules, excludes...)
			}

			slog.Debug("Initializing Walker",
				"walk_root", walkRoot,
				"base_rules_count", len(baseRules),
				"override_rules_count", len(overrideRules),
				"is_forced", isForced,
			)

			walker := lx.NewWalker(baseRules, overrideRules)
			walker.IgnoreEnabled = cfg.IgnoreEnabled
			walker.OnIgnore = func(p, reason string) {
				slog.Debug("Ignored", "path", p, "reason", reason)
			}

			count := 0

			err = walker.Walk(fsys, walkRoot, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					slog.Warn("Error accessing path during walk", "path", path, "error", err)
					return nil
				}
				if d.IsDir() {
					return nil
				}

				// Reconstruct display path relative to user input
				var effectivePath string
				if !stat.IsDir() {
					if displayPrefix != "" {
						effectivePath = filepath.Join(displayPrefix, filepath.FromSlash(path))
					} else {
						effectivePath = filepath.FromSlash(path)
					}
				} else {
					if path == "." {
						effectivePath = displayPrefix
					} else {
						effectivePath = filepath.Join(displayPrefix, filepath.FromSlash(path))
					}
				}

				// Skip directory symlinks to avoid recursion issues and IO errors
				if (d.Type() & fs.ModeSymlink) != 0 {
					if cfg.IgnoreFileSymlinks {
						return nil
					}
					targetInfo, err := fs.Stat(fsys, path)
					if err == nil && targetInfo.IsDir() {
						slog.Debug("Skipping directory symlink", "path", effectivePath)
						return nil
					}
				}

				// Expand archives inline before include filtering
				if cfg.ExpandArchives && lx.IsArchivePath(path) {
					var archiveAbsPath string
					if stat.IsDir() {
						archiveAbsPath = filepath.Join(absPath, filepath.FromSlash(path))
					} else {
						archiveAbsPath = absPath
					}
					archiveWalker := newArchiveWalker(cfg, isForced)
					archiveWalker.OnIgnore = func(p, reason string) {
						slog.Debug("Ignored in archive", "path", effectivePath+"/"+p, "reason", reason)
					}
					archiveIncludes := includes
					if isForced {
						archiveIncludes = nil
					}
					if err := lx.ExpandArchive(ctx, archiveAbsPath, effectivePath, archiveWalker, archiveIncludes, outPath, stream); err != nil {
						slog.Error("Failed to expand archive", "path", effectivePath, "error", err)
					}
					return nil
				}

				// Post-walk filtering for includes (weak filter, respects .gitignore)
				if !isForced && len(includes) > 0 {
					matched := false
					for _, inc := range includes {
						if lx.IsMatch(inc, path) {
							matched = true
							break
						}
					}
					if !matched {
						slog.Debug("Ignored by include filter (-i)", "path", effectivePath)
						return nil
					}
				}

				if outPath != "" {
					if abs, _ := filepath.Abs(effectivePath); abs == outPath {
						slog.Warn("Skipping output file to avoid infinite recursion", "path", effectivePath)
						return nil
					}
				}

				info, err := d.Info()
				if err != nil {
					slog.Error("Failed to stat file in walk", "path", path, "error", err)
					return nil
				}

				f := lx.NewInputFile(fsys, path, info)
				f.Path = effectivePath

				if !stat.IsDir() {
					if displayPrefix != "" {
						f.AbsPath = filepath.Join(filepath.Dir(absPath), path)
					} else {
						f.AbsPath = absPath
					}
				} else {
					f.AbsPath = filepath.Join(absPath, path)
				}

				slog.Debug("File accepted by walker", "path", f.Path, "size", f.Size)
				stream.AddFile(f)
				count++
				return nil
			})

			if err != nil {
				slog.Error("Walker traversal failed", "error", err)
			}
			slog.Debug("Walker finished", "root", rawPath, "files_found", count)

		case "tree", "tree-only":
			if body, ok := treePrecomp.strings[i]; ok {
				slog.Debug("Adding tree", "lines", strings.Count(body, "\n")+1)
				stream.AddTree(body)
			} else {
				slog.Debug("Skipping empty tree (no files in group)")
			}
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

func setupLogger(level slog.Level) {
	var handler slog.Handler
	if level > slog.LevelDebug {
		handler = NewCliHandler(os.Stderr, level)
	} else {
		handler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: level,
		})
	}
	slog.SetDefault(slog.New(handler))
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

	if _, ok := globals["expand"]; ok {
		slog.Debug("Override: Archive expansion enabled via flag")
		c.ExpandArchives = true
	}
	if _, ok := globals["no-expand"]; ok {
		slog.Debug("Override: Archive expansion disabled via flag")
		c.ExpandArchives = false
	}

	if _, ok := globals["documents"]; ok {
		slog.Debug("Override: Document extraction enabled via flag")
		c.ExtractDocuments = true
	}
	if _, ok := globals["no-documents"]; ok {
		slog.Debug("Override: Document extraction disabled via flag")
		c.ExtractDocuments = false
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

	movedDetails := make([]string, 0, len(modifiers))
	for _, m := range modifiers {
		desc := "--" + m.Action
		if m.Value != "" && m.Value != "true" {
			desc += fmt.Sprintf("=%q", m.Value)
		}
		movedDetails = append(movedDetails, desc)
	}

	slog.Warn("Trailing state modifiers detected; reordering them to apply to preceding files",
		"moved_flags", strings.Join(movedDetails, ", "))

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
