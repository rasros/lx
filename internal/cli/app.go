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
	"runtime"
	"strconv"
	"strings"
	"time"

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

	return processStream(ctx, parsed, args)
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

	if _, ok := parsed.Globals["list-prompts"]; ok {
		if err := runListPrompts(parsed); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return true
	}
	return false
}

func gatherInputs(parsed *ParsedArgs) error {
	// Stdin depends only on explicit file arguments; the "." fallback also
	// depends on generators, which already give the run something to emit.
	hasFiles := false
	hasGenerators := false
	for _, op := range parsed.Ops {
		switch op.Action {
		case "FILE", "file":
			hasFiles = true
		case "section", "prompt", "prompt-file", "system-context":
			hasGenerators = true
		}
	}
	if hasFiles {
		slog.Debug("Detected input from file actions")
	}

	if !hasFiles {
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
			hasFiles = true
		} else {
			slog.Debug("No stdin pipe detected")
		}
	}

	if !hasFiles && !hasGenerators {
		slog.Debug("No inputs detected, defaulting to current directory '.'")
		parsed.Ops = append(parsed.Ops, Op{Action: "FILE", Value: ".", Type: CmdAction})
	}
	return nil
}

func processStream(ctx context.Context, parsed *ParsedArgs, rawArgs []string) error {
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

	slog.Debug("Configuration loaded", "format", cfg.OutputFormat)

	applyOutputFormatOverrides(cfg, parsed)

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
		Head: -1,
		Tail: 0,
	}
	stream, err := lx.NewStream(cfg, defaultRunCfg)
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

	slog.Debug("Processing operations", "total_ops", len(parsed.Ops))

	globalIgnoreRules := LoadGlobalIgnorePatterns()
	promptResolver := newPromptResolver(resolvePromptsDir(parsed, cliOpts), cliOpts.PromptExtensions)
	sections := parseSections(parsed.Ops, defaultRunCfg)
	applyWorkloadConcurrency(stream, sections, runtime.NumCPU())
	precomputeTrees(ctx, sections, globalIgnoreRules)
	debugLoggingEnabled := slog.Default().Enabled(ctx, slog.LevelDebug)

	var metaFields map[string]string
	var metaBody string
	resolveSystemContext := func() (map[string]string, string) {
		if metaFields == nil {
			metaFields, metaBody = systemContext(rawArgs, time.Now())
		}
		return metaFields, metaBody
	}

	for si, section := range sections {
		slog.Debug("Processing section", "index", si, "ops", len(section.Ops))
		stream.WithRunnerConfig(section.RunCfg)
		includeSpecs := lx.CompileSpecs(section.Includes)

		for oi, op := range section.Ops {
			slog.Debug("Processing op", "action", op.Action, "value", op.Value)

			switch op.Action {
			case "FILE", "file":
				if section.skipFileOps[oi] {
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

				forceExpand := false
				if rewritten, ok := lx.RewriteRepoURL(rawPath); ok {
					slog.Debug("Rewrote repo URL to archive", "from", rawPath, "to", rewritten)
					rawPath = rewritten
					forceExpand = true
				}

				if lx.IsHTTPURL(rawPath) {
					if (section.RunCfg.ExpandArchives || forceExpand) && lx.IsHTTPArchiveURL(rawPath) {
						if !isForced && !lx.IsKept(rawPath, nil, section.Excludes) {
							slog.Debug("Skipping URL archive due to exclude filter", "url", rawPath)
							continue
						}

						tempPath, cleanup, err := lx.DownloadURLToTempFile(ctx, rawPath)
						if err != nil {
							slog.Error("Failed to download URL archive", "url", rawPath, "error", err)
							continue
						}
						tempCleanups = append(tempCleanups, cleanup)

						archiveWalker := newArchiveWalker(section.RunCfg.ShowHidden, isForced)
						if debugLoggingEnabled {
							archiveWalker.OnIgnore = func(p, reason string) {
								slog.Debug("Ignored in URL archive", "path", rawPath+"/"+p, "reason", reason)
							}
						}
						archiveIncludes := section.Includes
						if isForced {
							archiveIncludes = nil
						}
						if err := lx.ExpandArchive(ctx, tempPath, rawPath, archiveWalker, archiveIncludes, outPath, stream); err != nil {
							slog.Error("Failed to expand URL archive", "url", rawPath, "error", err)
						}
						continue
					}

					if !isForced && !lx.IsKept(rawPath, section.Includes, section.Excludes) {
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
					if !isForced && !lx.IsKept(rawPath, section.Includes, section.Excludes) {
						slog.Debug("Skipping missing path due to filters", "path", rawPath)
						continue
					}
					slog.Error("Failed to stat path", "path", absPath, "error", err)
					continue
				}

				if !stat.IsDir() {
					isExpandableArchive := section.RunCfg.ExpandArchives && lx.IsArchivePath(rawPath)
					if !isForced && !isExpandableArchive && !lx.IsKept(rawPath, section.Includes, section.Excludes) {
						slog.Debug("Skipping file due to filters", "path", rawPath)
						continue
					}
					if !isForced && isExpandableArchive && !lx.IsKept(rawPath, nil, section.Excludes) {
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

				if !section.RunCfg.NoIgnore {
					baseRules = append(baseRules, globalIgnoreRules...)
				}

				if !isForced {
					overrideRules = append(overrideRules, section.Excludes...)
				}

				slog.Debug("Initializing Walker",
					"walk_root", walkRoot,
					"base_rules_count", len(baseRules),
					"override_rules_count", len(overrideRules),
					"is_forced", isForced,
				)

				walker := lx.NewWalker(baseRules, overrideRules)
				walker.IgnoreEnabled = !section.RunCfg.NoIgnore
				walker.SkipHidden = !section.RunCfg.ShowHidden && !isForced
				if debugLoggingEnabled {
					walker.OnIgnore = func(p, reason string) {
						slog.Debug("Ignored", "path", p, "reason", reason)
					}
				}

				count := 0

				err = walker.Walk(fsys, walkRoot, func(path string, d fs.DirEntry, err error) error {
					if err != nil {
						slog.Warn("Error accessing path during walk", "path", path, "error", err)
						return nil
					}
					if d.IsDir() {
						if !isForced && len(includeSpecs) > 0 && path != "." && !lx.CouldMatchAnyDescendant(includeSpecs, path) {
							return fs.SkipDir
						}
						return nil
					}

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

					if (d.Type() & fs.ModeSymlink) != 0 {
						if section.RunCfg.SkipFileSymlinks {
							return nil
						}
						targetInfo, err := fs.Stat(fsys, path)
						if err == nil && targetInfo.IsDir() {
							if !section.RunCfg.FollowDirSymlinks {
								slog.Debug("Skipping directory symlink", "path", effectivePath)
								return nil
							}
						}
					}

					if section.RunCfg.ExpandArchives && lx.IsArchivePath(path) {
						var archiveAbsPath string
						if stat.IsDir() {
							archiveAbsPath = filepath.Join(absPath, filepath.FromSlash(path))
						} else {
							archiveAbsPath = absPath
						}
						archiveWalker := newArchiveWalker(section.RunCfg.ShowHidden, isForced)
						if debugLoggingEnabled {
							archiveWalker.OnIgnore = func(p, reason string) {
								slog.Debug("Ignored in archive", "path", effectivePath+"/"+p, "reason", reason)
							}
						}
						archiveIncludes := section.Includes
						if isForced {
							archiveIncludes = nil
						}
						if err := lx.ExpandArchive(ctx, archiveAbsPath, effectivePath, archiveWalker, archiveIncludes, outPath, stream); err != nil {
							slog.Error("Failed to expand archive", "path", effectivePath, "error", err)
						}
						return nil
					}

					if !isForced && len(includeSpecs) > 0 {
						if !lx.IsMatchAnyCompiled(includeSpecs, path) {
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
				slog.Debug("Walker finished", "root", rawPath, "files_matched", count)

			case "tree", "tree-only":
				if ts, ok := section.treeStrings[oi]; ok {
					slog.Debug("Adding tree", "lines", strings.Count(ts, "\n")+1)
					stream.AddTree(ts)
				} else {
					slog.Debug("Skipping empty tree (no files in group)")
				}
			case "section":
				slog.Debug("Adding section", "title", op.Value)
				stream.AddSection(op.Value)
			case "system-context":
				fields, body := resolveSystemContext()
				slog.Debug("Adding system context", "fields", len(fields))
				stream.AddMeta(body, fields)
			case "prompt":
				slog.Debug("Adding prompt", "length", len(op.Value))
				stream.AddPrompt(op.Value)
			case "prompt-file":
				path, err := promptResolver.resolve(op.Value)
				if err != nil {
					return err
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return fmt.Errorf("read prompt file %s: %w", path, err)
				}
				slog.Debug("Adding prompt from file", "path", path, "length", len(data))
				stream.AddPrompt(string(data))
			}
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

const maxSkeletonWorkers = 2

func applyWorkloadConcurrency(stream *lx.Stream, sections []Section, cpuCount int) {
	workers, limited := workloadConcurrency(sections, cpuCount)
	if !limited {
		return
	}
	stream.WithConcurrency(workers)
	slog.Debug("Applying skeleton concurrency limit", "workers", workers, "cpu_count", cpuCount)
}

func workloadConcurrency(sections []Section, cpuCount int) (int, bool) {
	if cpuCount < 1 {
		cpuCount = 1
	}

	for _, section := range sections {
		if section.RunCfg.SkeletonFunctions || section.RunCfg.SkeletonTypes {
			if cpuCount > maxSkeletonWorkers {
				return maxSkeletonWorkers, true
			}
			return cpuCount, true
		}
	}
	return 0, false
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
		if hasCopy || hasOutput {
			show = true
		}
	}

	if show {
		err := stream.GetEngine().Stats.Execute(debugOut, lx.StatsContext{
			Global:       stream.GetGlobalContext(),
			ColorEnabled: shouldColorStats(debugOut),
		})
		if err != nil {
			slog.Error("Failed to render stats", "error", err)
		}
	}
}

// shouldColorStats honors NO_COLOR / CLICOLOR / FORCE_COLOR / TERM so AI agents
// and CI runners that capture stdout/stderr through pipes get clean output.
func shouldColorStats(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("CLICOLOR") == "0" {
		return false
	}
	term := os.Getenv("TERM")
	if term == "" || term == "dumb" {
		return false
	}
	if os.Getenv("CLICOLOR_FORCE") != "" || os.Getenv("FORCE_COLOR") != "" {
		return true
	}
	var f *os.File
	switch w {
	case os.Stderr:
		f = os.Stderr
	case os.Stdout:
		f = os.Stdout
	default:
		return false
	}
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
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

func applyOutputFormatOverrides(cfg *lx.Config, parsed *ParsedArgs) {
	for _, op := range parsed.Ops {
		switch op.Action {
		case "md":
			cfg.OutputFormat = "markdown"
		case "xml":
			cfg.OutputFormat = "xml"
		case "html":
			cfg.OutputFormat = "html"
		case "bare":
			cfg.OutputFormat = "bare"
		}
	}
}

func determineOutput(globals map[string]string, defaultMode string) (io.Writer, *bytes.Buffer, io.Writer, error) {
	outputPath, hasOutput := globals["output"]
	_, hasCopy := globals["copy"]
	_, hasStdout := globals["stdout"]

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
	} else if hasStdout {
		slog.Debug("Output explicitly forced to stdout")
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
