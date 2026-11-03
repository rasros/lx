package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/rasros/lx/internal/ansi"
	"github.com/rasros/lx/pkg/lx"
)

// Run runs lx with args against env, which stands for the process: Run reads
// and writes nothing else of it, and logs to env.Stderr through a logger of
// its own. A run that ctx cancels returns ctx's error without logging it.
func Run(ctx context.Context, args []string, env Env) error {
	parsed, err := Parse(args, definitions)
	if err != nil {
		return fmt.Errorf("argument parsing failed: %w", err)
	}

	stopProfiling, err := setupProfiling(parsed, env)
	if err != nil {
		return fmt.Errorf("profiling setup failed: %w", err)
	}
	defer stopProfiling()

	if done, err := handleGlobals(parsed, env); done || err != nil {
		return err
	}

	if err := gatherInputs(parsed, env); err != nil {
		if len(args) == 0 {
			printShortHelp(env)
			return nil
		}
		return fmt.Errorf("input gathering failed: %w", err)
	}

	return processStream(ctx, parsed, args, env)
}

// handleGlobals runs the options that replace a run, reporting whether one
// did.
func handleGlobals(parsed *ParsedArgs, env Env) (bool, error) {
	for _, op := range parsed.Ops {
		if op.Action == "help" {
			if op.IsShort {
				printShortHelp(env)
			} else {
				printLongHelp(env)
			}
			return true, nil
		}
	}

	if _, ok := parsed.Globals["version"]; ok {
		fmt.Fprintf(env.Stdout, "lx version %s\n", Version)
		return true, nil
	}

	if _, ok := parsed.Globals["list-prompts"]; ok {
		if err := runListPrompts(parsed, env); err != nil {
			fmt.Fprintln(env.Stderr, err)
			return true, ExitError{Code: 1}
		}
		return true, nil
	}
	return false, nil
}

// gatherInputs adds the paths piped to stdin when no file is named, and "."
// when there is still nothing to render.
func gatherInputs(parsed *ParsedArgs, env Env) error {
	// Stdin depends only on explicit file arguments; the "." fallback also
	// depends on generators, which already give the run something to emit.
	hasFiles := false
	hasGenerators := false
	for _, op := range parsed.Ops {
		switch op.Action {
		case "FILE":
			hasFiles = true
		case "section", "prompt", "prompt-file", "system-context":
			hasGenerators = true
		}
	}

	if !hasFiles {
		_, useNull := parsed.Globals["null"]
		stdinFiles, isPipe, err := readFilenamesFromStdin(env.Stdin, useNull)
		if err != nil {
			return fmt.Errorf("read stdin: %w", err)
		}
		if isPipe {
			for _, f := range stdinFiles {
				parsed.Ops = append(parsed.Ops, Op{Action: "FILE", Value: f, Type: CmdAction, Piped: true})
			}
			hasFiles = true
		}
	}

	if !hasFiles && !hasGenerators {
		parsed.Ops = append(parsed.Ops, Op{Action: "FILE", Value: ".", Type: CmdAction})
	}
	return nil
}

func processStream(ctx context.Context, parsed *ParsedArgs, rawArgs []string, env Env) error {
	initialLevel, err := determineLogLevel(parsed, "warn")
	if err != nil {
		return err
	}
	log := newLogger(env, initialLevel)
	log.Debug("Logger initialized (early)", "level", initialLevel.String())

	cfg, cliOpts, err := LoadConfigChain(env, log, parsed.Globals["config"])
	if err != nil {
		return err
	}

	finalLevel, err := determineLogLevel(parsed, cliOpts.Verbosity)
	if err != nil {
		return err
	}

	if finalLevel != initialLevel {
		log = newLogger(env, finalLevel)
		log.Debug("Logger level updated from config", "new_level", finalLevel.String())
	}

	log.Debug("Configuration loaded", "format", cfg.OutputFormat)

	applyOutputFormatOverrides(cfg, parsed)

	out := determineOutput(parsed.Globals, cliOpts.OutputMode, env, log)
	w := out.w
	var sink *fileSink
	if out.path != "" {
		// A bad path fails before any work, but the file only receives the
		// output once all of it is written, so a failed run leaves it as it was.
		if sink, err = createFileSink(out.path, out.name, env.TempDir); err != nil {
			return err
		}
		defer sink.Abort()
		w = sink
	}

	stream, err := lx.NewStream(cfg)
	if err != nil {
		return err
	}

	stream.WithLogger(log).WithDir(env.Cwd).WithOnFailure(func(f lx.Failure) {
		logFailure(log, f)
	})
	log.Debug("Processing operations", "total_ops", len(parsed.Ops))

	ignoreRules, gitExcludes := lx.UserIgnoreRules(env.LookupEnv, log)
	promptResolver := newPromptResolver(env, resolvePromptsDir(parsed, cliOpts, env), cliOpts.PromptExtensions)
	sections := parseSections(log, parsed.Ops, lx.RenderOptions{Head: -1})
	for i := range sections {
		d := &sections[i].Discover
		d.IgnoreRules, d.GitExcludes, d.OutPath = ignoreRules, gitExcludes, out.path
	}

	if err := resolveSections(ctx, sections, lx.NewCollector(env.Cwd, log)); err != nil {
		return err
	}
	assignTrees(log, sections)
	var metaFields map[string]string
	var metaBody string
	resolveSystemContext := func() (map[string]string, string) {
		if metaFields == nil {
			metaFields, metaBody = systemContext(rawArgs, time.Now())
		}
		return metaFields, metaBody
	}

	for si, section := range sections {
		log.Debug("Processing section", "index", si, "ops", len(section.Ops))

		for oi, op := range section.Ops {
			log.Debug("Processing op", "action", op.Action, "value", op.Value)

			switch op.Action {
			case "FILE":
				for _, f := range section.failures[oi] {
					stream.AddFailure(f)
				}
				if section.skipFileOps[oi] {
					log.Debug("Skipping file content (tree-only mode)", "path", op.Value)
					continue
				}

				if op.Value == "-" {
					log.Info("Reading content from stdin")
					data, err := io.ReadAll(env.Stdin)
					if err != nil {
						stream.AddFailure(lx.Failure{Kind: lx.FailedRead, Path: stdinPath, Display: "stdin", Err: err})
						continue
					}
					stream.AddFile(lx.NewBufferInputFile("stdin", data), section.Render)
					continue
				}

				for _, f := range section.resolved[oi] {
					stream.AddFile(f, section.Render)
				}

			case "tree", "tree-only":
				stream.AddTree(section.treeFiles[oi], section.Render)
			case "section":
				log.Debug("Adding section", "title", op.Value)
				stream.AddSection(op.Value)
			case "system-context":
				fields, body := resolveSystemContext()
				log.Debug("Adding system context", "fields", len(fields))
				stream.AddMeta(body, fields)
			case "prompt":
				log.Debug("Adding prompt", "length", len(op.Value))
				stream.AddPrompt(op.Value)
			case "prompt-file":
				path, err := promptResolver.resolve(op.Value)
				if err != nil {
					return err
				}
				data, err := env.readFile(path)
				if err != nil {
					return fmt.Errorf("read prompt file %s: %w", path, err)
				}
				log.Debug("Adding prompt from file", "path", path, "length", len(data))
				stream.AddPrompt(string(data))
			}
		}
	}

	if sink != nil {
		log.Info("Writing output to file", "path", out.name)
	}

	log.Info("Executing stream pipeline...")
	err = stream.Execute(ctx, w)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return err
	}
	if sink != nil {
		if err := sink.Commit(); err != nil {
			return err
		}
	}

	if out.clip != nil {
		log.Info("Copying output to clipboard", "bytes", out.clip.Len())
		if env.Clipboard == nil {
			return errors.New("clipboard write failed: no clipboard")
		}
		if err := env.Clipboard(out.clip.String()); err != nil {
			return fmt.Errorf("clipboard write failed: %w", err)
		}
		log.Info("Clipboard copy successful")
	}

	global := stream.GetGlobalContext()
	if global.TotalFiles == 0 && global.SkippedBySize > 0 {
		log.Warn("Every input was skipped by the size limit (-m)",
			"skipped", global.SkippedBySize)
	}

	handleStatsDisplay(log, env, parsed, cliOpts, stream, out.debug)
	return nil
}

// newLogger returns the logger of a run at level, writing to env.Stderr.
func newLogger(env Env, level slog.Level) *slog.Logger {
	if level > slog.LevelDebug {
		return slog.New(NewCliHandler(env.Stderr, level, env.paint(env.Stderr)))
	}
	return slog.New(slog.NewTextHandler(env.Stderr, &slog.HandlerOptions{Level: level}))
}

func handleStatsDisplay(log *slog.Logger, env Env, parsed *ParsedArgs, cliOpts *CliConfig, stream *lx.Stream, debugOut io.Writer) {
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
		global := stream.GetGlobalContext()
		err := stream.GetEngine().Stats.Execute(debugOut, lx.StatsContext{
			Global:       global,
			ColorEnabled: ansi.Enabled(debugOut, env.LookupEnv),
		})
		if err != nil {
			log.Error("Failed to render stats", "error", err)
		}
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

// output is where a run writes its bundle and its stats.
type output struct {
	// w receives the bundle, unless it goes to the file at path.
	w io.Writer
	// path is the absolute path of the output file, if any, and name that
	// path as given.
	path, name string
	// clip, when set, buffers the bundle for the clipboard.
	clip *bytes.Buffer
	// debug receives the stats.
	debug io.Writer
}

func determineOutput(globals map[string]string, defaultMode string, env Env, log *slog.Logger) output {
	outputPath, hasOutput := globals["output"]
	_, hasCopy := globals["copy"]
	_, hasStdout := globals["stdout"]

	out := output{w: env.Stdout, debug: env.Stderr}

	if hasOutput {
		out.path, out.name = env.abs(outputPath), outputPath
		out.w = nil
		out.debug = env.Stdout
		log.Debug("Output set to file", "path", outputPath)
		log.Debug("Output file path resolved", "path", out.path)
	} else if hasStdout {
		log.Debug("Output explicitly forced to stdout")
	} else if hasCopy || defaultMode == "copy" {
		out.clip = new(bytes.Buffer)
		out.w = out.clip
		out.debug = env.Stdout
		log.Debug("Output set to clipboard buffer")
	} else {
		log.Debug("Output set to stdout")
	}

	return out
}
