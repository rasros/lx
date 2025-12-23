package lx

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"

	ucli "github.com/urfave/cli/v3"
)

var Version = "(devel)"

func init() {
	// If ldflags already set Version (e.g. release builds), leave it unchanged.
	if Version != "(devel)" {
		return
	}

	// For go install
	if info, ok := debug.ReadBuildInfo(); ok {
		v := info.Main.Version
		if v != "" && v != "(devel)" {
			Version = v
		}
	}
}

// NewCommand builds the urfave/cli command for lx.
func NewCommand() *ucli.Command {
	var opts Options

	// Make --help the only help flag (freeing -h for --head).
	ucli.HelpFlag = &ucli.BoolFlag{
		Name:        "help",
		Usage:       "show help",
		HideDefault: true,
		Local:       true,
	}

	return &ucli.Command{
		Name:    "lx",
		Usage:   "print files with headers, delimiters, and optional head/tail slicing",
		Version: Version,

		Flags: []ucli.Flag{
			&ucli.IntFlag{
				Name:        "head",
				Aliases:     []string{"h"},
				Usage:       "print first N lines (0 = no limit)",
				Destination: &opts.Head,
			},

			&ucli.IntFlag{
				Name:        "tail",
				Aliases:     []string{"t"},
				Usage:       "print last N lines (0 = no limit)",
				Destination: &opts.Tail,
			},

			&ucli.IntFlag{
				Name:        "n",
				Usage:       "print N lines split between head and tail (0 = no limit)",
				Destination: &opts.NBoth,
			},

			&ucli.StringFlag{
				Name: "prefix-delimiter",
				Usage: "string printed before file contents; placeholders: {filename}, {row_count}, " +
					"{byte_size}, {last_modified}, {language}, {n}",
				Destination: &opts.PrefixDelimiter,
			},
			&ucli.StringFlag{
				Name:        "postfix-delimiter",
				Usage:       "string printed after file contents, see prefix-delimter for placeholders",
				Destination: &opts.PostfixDelimiter,
			},

			&ucli.BoolFlag{
				Name:        "line-numbers",
				Aliases:     []string{"l"},
				Usage:       "print line numbers",
				Destination: &opts.LineNumbers,
			},
			&ucli.BoolFlag{
				Name:        "tokens",
				Usage:       "print estimated token count to stderr",
				Destination: &opts.ShowTokens,
			},
		},

		Action: func(ctx context.Context, cmd *ucli.Command) error {
			// Track which flags were explicitly set to preserve override rules.
			opts.HeadSet = cmd.IsSet("head")
			opts.TailSet = cmd.IsSet("tail")
			opts.NSet = cmd.IsSet("n")

			// Start with filenames from CLI args.
			files := cmd.Args().Slice()

			// Add filenames from piped stdin (one per line), if any.
			stdinFiles, err := readFilenamesFromStdin()
			if err != nil {
				return fmt.Errorf("lx: read stdin: %w", err)
			}
			if len(stdinFiles) > 0 {
				files = append(files, stdinFiles...)
			}

			if len(files) == 0 {
				return fmt.Errorf("lx: provide one or more file paths via args or stdin")
			}

			r := opts.Effective()

			if err := r.Run(files, os.Stdout); err != nil {
				return fmt.Errorf("lx: %w", err)
			}
			return nil
		},
	}
}
