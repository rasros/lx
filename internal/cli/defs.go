package cli

import (
	"runtime/debug"
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

var definitions = []CommandDef{
	// Misc flags
	{Name: "config", Short: "y", Type: CmdGlobal, ValueType: ValueAny, Usage: "Load configuration from a specific YAML file. Overrides defaults (~/.config/lx/config.yaml) and env vars."},
	{Name: "version", Short: "V", Type: CmdGlobal, ValueType: ValueNone, Usage: "Print version information and exit."},
	{Name: "help", Short: "h", Type: CmdGlobal, ValueType: ValueNone, Usage: "Show this help message."},

	// Logging
	{Name: "quiet", Short: "q", Type: CmdGlobal, ValueType: ValueNone, Usage: "Suppress standard output noise. Hides the token/file summary usually printed to stderr."},
	{Name: "verbose", Type: CmdGlobal, ValueType: ValueAny, Usage: "Explicitly set log level ('debug', 'info', 'warn', 'error', 'silent')."},
	{Name: "verbosity", Short: "v", Type: CmdGlobal, ValueType: ValueCounter, Usage: "Increase log verbosity (stackable: -v for info, -vv for debug)."},

	// Stats Control
	{Name: "stats", Type: CmdGlobal, ValueType: ValueNone, Usage: "Force display of the file/token summary to stderr, even when piping output to stdout."},
	{Name: "no-stats", Type: CmdGlobal, ValueType: ValueNone, Usage: "Disable the file/token summary footer entirely."},

	{Name: "null", Short: "0", Type: CmdGlobal, ValueType: ValueNone, Usage: "Read NUL-terminated filenames from stdin (compatible with 'find . -print0' for paths with spaces)."},

	// Format flags
	{Name: "md", Type: CmdGlobal, ValueType: ValueNone, Usage: "Format output as Markdown with code fences (default). Best for GPT-4 and general LLMs."},
	{Name: "xml", Type: CmdGlobal, ValueType: ValueNone, Usage: "Format output as XML tags. Recommended for Anthropic's Claude to improve context parsing."},
	{Name: "html", Type: CmdGlobal, ValueType: ValueNone, Usage: "Format output as a standalone HTML file with syntax highlighting. Best for human review/sharing."},

	// Profiling
	{Name: "cpuprofile", Type: CmdGlobal, ValueType: ValueAny, Usage: "Write CPU profile to file.", Internal: true},
	{Name: "memprofile", Type: CmdGlobal, ValueType: ValueAny, Usage: "Write memory profile to file.", Internal: true},

	// Output format
	{Name: "copy", Short: "c", Type: CmdGlobal, ValueType: ValueNone, Usage: "Copy the final formatted output directly to the system clipboard."},
	{Name: "stdout", Short: "C", Type: CmdGlobal, ValueType: ValueNone, Usage: "Force output to stdout, overriding 'output_mode: copy' in configuration."},
	{Name: "output", Short: "o", Type: CmdGlobal, ValueType: ValueAny, Usage: "Write the formatted result to the specified file path instead of stdout."},

	// Global flags
	{Name: "follow", Short: "L", Type: CmdGlobal, ValueType: ValueNone, Usage: "Follow symbolic links during recursive directory walking."},
	{Name: "hidden", Short: "H", Type: CmdGlobal, ValueType: ValueNone, Usage: "Include hidden files and directories (dotfiles) in discovery."},
	{Name: "ignore", Type: CmdGlobal, ValueType: ValueNone, Usage: "Enable respect for .gitignore, .ignore, and .lxignore files (default behavior)."},
	{Name: "no-ignore", Short: "I", Type: CmdGlobal, ValueType: ValueNone, Usage: "Disregard all ignore files; traverse every discovered file and directory."},
	{Name: "no-follow", Type: CmdGlobal, ValueType: ValueNone, Usage: "Do not follow symbolic links (default)."},
	{Name: "no-hidden", Type: CmdGlobal, ValueType: ValueNone, Usage: "Skip hidden files (default)."},

	// Interleaved flags
	{Name: "line-numbers", Short: "l", Type: CmdInterleaved, ValueType: ValueNone, Usage: "Enable line numbers for subsequent files. Helps LLMs reference specific lines in their response."},
	{Name: "no-line-numbers", Short: "L", Type: CmdInterleaved, ValueType: ValueNone, Usage: "Disable line numbers for subsequent files."},
	{Name: "head", Type: CmdInterleaved, ValueType: ValueNumber, Usage: "Read only the first N lines of subsequent files."},
	{Name: "tail", Type: CmdInterleaved, ValueType: ValueNumber, Usage: "Read only the last N lines of subsequent files."},
	{Name: "lines", Short: "n", Type: CmdInterleaved, ValueType: ValueNumber, Usage: "Limit subsequent files to N lines (split between head/tail). Set 0 for 'compact mode' (filename only)."},
	{Name: "reset-lines", Short: "N", Type: CmdInterleaved, ValueType: ValueNone, Usage: "Reset slicing rules; print full content for subsequent files."},

	// Filter flags
	{Name: "include", Short: "i", Type: CmdInterleaved, ValueType: ValueAny, Usage: "Add a glob pattern to the whitelist. Only matching files are processed (OR logic)."},
	{Name: "exclude", Short: "e", Type: CmdInterleaved, ValueType: ValueAny, Usage: "Add a glob pattern to the blacklist. Overrides includes."},
	{Name: "reset-filters", Short: "E", Type: CmdInterleaved, ValueType: ValueNone, Usage: "Clear active include/exclude filters. Subsequent files are not filtered."},

	// Actions
	{Name: "file", Short: "f", Type: CmdAction, ValueType: ValueAny, Usage: "Process a specific path immediately, bypassing global discovery depth limits."},
	{Name: "section", Short: "s", Type: CmdAction, ValueType: ValueAny, Usage: "Insert a logical header/separator and reset the file counter. Useful for grouping distinct contexts."},
	{Name: "prompt", Short: "p", Type: CmdAction, ValueType: ValueAny, Usage: "Inject a custom text prompt or instruction directly into the output stream."},
}
