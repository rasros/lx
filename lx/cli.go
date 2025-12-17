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
	if Version != "(devel)" {
		return
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		v := info.Main.Version
		if v != "" && v != "(devel)" {
			Version = v
		}
	}
}

func NewCommand() *ucli.Command {
	var opts Options

	ucli.HelpFlag = &ucli.BoolFlag{
		Name:        "help",
		Usage:       "show help",
		HideDefault: true,
		Local:       true,
	}

	return &ucli.Command{
		Name:    "lx",
		Usage:   "print files with headers, slicing, and go-templates",
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
				Usage:       "print N lines split between head and tail",
				Destination: &opts.NBoth,
			},
			&ucli.StringFlag{
				Name:        "config",
				Aliases:     []string{"f"},
				Usage:       "path to yaml config file",
				Destination: &opts.ConfigPath,
			},
			&ucli.BoolFlag{
				Name:        "line-numbers",
				Aliases:     []string{"l"},
				Usage:       "print line numbers",
				Destination: &opts.LineNumbers,
			},
		},

		Action: func(ctx context.Context, cmd *ucli.Command) error {
			opts.HeadSet = cmd.IsSet("head")
			opts.TailSet = cmd.IsSet("tail")
			opts.NSet = cmd.IsSet("n")

			files := cmd.Args().Slice()

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

			r, err := opts.Effective()
			if err != nil {
				return err
			}

			if err := r.Run(files, os.Stdout); err != nil {
				return fmt.Errorf("lx: %w", err)
			}
			return nil
		},
	}
}
