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
	{Name: "config", Short: "y", Type: CmdGlobal, ValueType: ValueAny, Usage: "Load configuration from a specific YAML file. Overrides default search paths (~/.config/lx/config.yaml) and LX_CONFIG environment variable."},
	{Name: "version", Short: "V", Type: CmdGlobal, ValueType: ValueNone, Usage: "Print the current version and exit."},
	{Name: "help", Short: "h", Type: CmdGlobal, ValueType: ValueNone, Usage: "Show this help message."},

	// Logging
	{Name: "quiet", Short: "q", Type: CmdGlobal, ValueType: ValueNone, Usage: "Silence all log output, including the stats summary. Errors will still be printed to stderr."},
	{Name: "verbose", Type: CmdGlobal, ValueType: ValueAny, Usage: "Set specific log level. Options: 'trace', 'debug', 'info', 'warn' (default), 'error', 'silent'."},
	{Name: "verbosity", Short: "v", Type: CmdGlobal, ValueType: ValueCounter, Usage: "Increase verbosity level. Stackable: -v (info), -vv (debug), -vvv (trace)."},

	// Stats Control
	{Name: "stats", Type: CmdGlobal, ValueType: ValueNone, Usage: "Force display of the summary (file count, size, token estimate) to stderr. By default, stats are shown only when output is redirected or copied."},
	{Name: "no-stats", Type: CmdGlobal, ValueType: ValueNone, Usage: "Disable the summary stats entirely, even when output is redirected."},

	{Name: "null", Short: "0", Type: CmdGlobal, ValueType: ValueNone, Usage: "Read input filenames from stdin as NUL-terminated strings. Useful for handling filenames with spaces when piping from 'find . -print0'."},

	// Format flags
	{Name: "md", Type: CmdGlobal, ValueType: ValueNone, Usage: "Output as Markdown (default). Wraps content in code blocks with language identifiers."},
	{Name: "xml", Type: CmdGlobal, ValueType: ValueNone, Usage: "Output as XML. Recommended for Anthropic's Claude to improve context parsing and adherence."},
	{Name: "html", Type: CmdGlobal, ValueType: ValueNone, Usage: "Output as a standalone HTML document with syntax highlighting. Suitable for sharing or viewing code in a browser."},

	// Profiling
	{Name: "cpuprofile", Type: CmdGlobal, ValueType: ValueAny, Usage: "Write CPU profile to the specified file for debugging performance.", Internal: true},
	{Name: "memprofile", Type: CmdGlobal, ValueType: ValueAny, Usage: "Write memory profile to the specified file for debugging memory usage.", Internal: true},

	// Output format
	{Name: "copy", Short: "c", Type: CmdGlobal, ValueType: ValueNone, Usage: "Copy the formatted output directly to the system clipboard. Supports pbcopy (macOS), xclip/wl-copy (Linux), and clip (Windows)."},
	{Name: "stdout", Short: "C", Type: CmdGlobal, ValueType: ValueNone, Usage: "Force writing to stdout. Default behavior, but useful to override config if 'output_mode' is set to 'copy'."},
	{Name: "output", Short: "o", Type: CmdGlobal, ValueType: ValueAny, Usage: "Write the formatted output to a specific file instead of stdout."},

	// Global flags
	{Name: "follow", Short: "L", Type: CmdGlobal, ValueType: ValueNone, Usage: "Follow symbolic links during recursive directory walking. Be careful with circular links."},
	{Name: "hidden", Short: "H", Type: CmdGlobal, ValueType: ValueNone, Usage: "Include hidden files and directories (starting with '.') in the search."},
	{Name: "ignore", Type: CmdGlobal, ValueType: ValueNone, Usage: "Respect .gitignore, .ignore, and .lxignore files (default: true)."},
	{Name: "no-ignore", Short: "I", Type: CmdGlobal, ValueType: ValueNone, Usage: "Disable ignore file logic; process all files regardless of gitignore rules."},
	{Name: "no-follow", Type: CmdGlobal, ValueType: ValueNone, Usage: "Do not follow symbolic links (default)."},
	{Name: "no-hidden", Type: CmdGlobal, ValueType: ValueNone, Usage: "Skip hidden files (default)."},

	// Interleaved flags
	{Name: "line-numbers", Short: "l", Type: CmdInterleaved, ValueType: ValueNone, Usage: "Enable line numbers for subsequent files. Highly recommended when pasting logs or long source files to an LLM for debugging."},
	{Name: "no-line-numbers", Short: "L", Type: CmdInterleaved, ValueType: ValueNone, Usage: "Disable line numbers for subsequent files."},
	{Name: "head", Type: CmdInterleaved, ValueType: ValueNumber, Usage: "Print first N lines of subsequent files."},
	{Name: "tail", Type: CmdInterleaved, ValueType: ValueNumber, Usage: "Print last N lines of subsequent files."},
	{Name: "lines", Short: "n", Type: CmdInterleaved, ValueType: ValueNumber, Usage: "Print N lines total for subsequent files, split evenly between the top and bottom of the file. Set to 0 for 'compact view' (lists filename/size only, no content)."},
	{Name: "reset-lines", Short: "N", Type: CmdInterleaved, ValueType: ValueNone, Usage: "Reset head/tail/lines limits to default (print entire file)."},

	// Filter flags
	{Name: "include", Short: "i", Type: CmdInterleaved, ValueType: ValueAny, Usage: "Add an include glob pattern. Can be used multiple times (OR logic). See GLOBBING section below for syntax details."},
	{Name: "exclude", Short: "e", Type: CmdInterleaved, ValueType: ValueAny, Usage: "Add an exclude glob pattern. Can be used multiple times. Matches are skipped."},
	{Name: "reset-filters", Short: "E", Type: CmdInterleaved, ValueType: ValueNone, Usage: "Clear all active include/exclude filters. Subsequent files will be processed without previous filters."},

	// Actions
	{Name: "file", Short: "f", Type: CmdAction, ValueType: ValueAny, Usage: "Explicitly add a file or directory path to the stream. Can be mixed with flags to apply settings to specific files."},
	{Name: "section", Short: "s", Type: CmdAction, ValueType: ValueAny, Usage: "Inject a section header (Markdown H2 / XML section) into the output stream."},
	{Name: "prompt", Short: "p", Type: CmdAction, ValueType: ValueAny, Usage: "Inject a custom text prompt or instruction into the output stream."},
}
