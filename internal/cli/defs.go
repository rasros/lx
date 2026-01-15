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
	{Name: "config", Short: "y", Type: CmdGlobal, ValueType: ValueAny, Usage: "Load configuration from a specific YAML file, overriding defaults and environment variables."},
	{Name: "version", Short: "V", Type: CmdGlobal, ValueType: ValueNone, Usage: "Print version information and exit."},
	{Name: "help", Short: "h", Type: CmdGlobal, ValueType: ValueNone, Usage: "Show this help message."},

	// Logging
	{Name: "quiet", Short: "q", Type: CmdGlobal, ValueType: ValueNone, Usage: "Suppress all non-error output. Hides the token/file summary usually printed to stderr."},
	{Name: "verbose", Type: CmdGlobal, ValueType: ValueAny, Usage: "Set log level: 'trace', 'debug', 'info', 'warn', 'error', 'silent'."},
	{Name: "verbosity", Short: "v", Type: CmdGlobal, ValueType: ValueCounter, Usage: "Increase log level (stackable: -v, -vv, -vvv)."},

	// Stats Control
	{Name: "stats", Type: CmdGlobal, ValueType: ValueNone, Usage: "Force display of the file/token summary to stderr, even when writing to stdout."},
	{Name: "no-stats", Type: CmdGlobal, ValueType: ValueNone, Usage: "Never display the file/token summary."},

	{Name: "null", Short: "0", Type: CmdGlobal, ValueType: ValueNone, Usage: "Expect NUL-terminated filenames from stdin. Compatible with 'find . -print0' to handle spaces in paths."},

	// Format flags
	{Name: "md", Type: CmdGlobal, ValueType: ValueNone, Usage: "Output Markdown (default). Files are wrapped in language-specific code blocks."},
	{Name: "xml", Type: CmdGlobal, ValueType: ValueNone, Usage: "Output XML. Optimized for Anthropic's Claude to improve parsing and context separation."},
	{Name: "html", Type: CmdGlobal, ValueType: ValueNone, Usage: "Output a standalone HTML document with syntax highlighting and Pico CSS. Ideal for browser viewing."},

	// Profiling
	{Name: "cpuprofile", Type: CmdGlobal, ValueType: ValueAny, Usage: "Write CPU profile to file.", Internal: true},
	{Name: "memprofile", Type: CmdGlobal, ValueType: ValueAny, Usage: "Write memory profile to file.", Internal: true},

	// Output format
	{Name: "copy", Short: "c", Type: CmdGlobal, ValueType: ValueNone, Usage: "Copy output to system clipboard. Auto-detects pbcopy, xclip, wl-copy, or clip.exe."},
	{Name: "stdout", Short: "C", Type: CmdGlobal, ValueType: ValueNone, Usage: "Force output to stdout, overriding 'output_mode: copy' in config."},
	{Name: "output", Short: "o", Type: CmdGlobal, ValueType: ValueAny, Usage: "Write output to the specified file path."},

	// Global flags
	{Name: "follow", Short: "L", Type: CmdGlobal, ValueType: ValueNone, Usage: "Follow symbolic links during directory traversal."},
	{Name: "hidden", Short: "H", Type: CmdGlobal, ValueType: ValueNone, Usage: "Include hidden files and directories (dotfiles) in the search."},
	{Name: "ignore", Type: CmdGlobal, ValueType: ValueNone, Usage: "Respect .gitignore, .ignore, and .lxignore files (default behavior)."},
	{Name: "no-ignore", Short: "I", Type: CmdGlobal, ValueType: ValueNone, Usage: "Disregard all ignore files; process every discovered file."},
	{Name: "no-follow", Type: CmdGlobal, ValueType: ValueNone, Usage: "Do not follow symbolic links (default)."},
	{Name: "no-hidden", Type: CmdGlobal, ValueType: ValueNone, Usage: "Skip hidden files (default)."},

	// Interleaved flags
	{Name: "line-numbers", Short: "l", Type: CmdInterleaved, ValueType: ValueNone, Usage: "Add line numbers to subsequent files. Useful for precise LLM referencing."},
	{Name: "no-line-numbers", Short: "L", Type: CmdInterleaved, ValueType: ValueNone, Usage: "Disable line numbers for subsequent files."},
	{Name: "head", Type: CmdInterleaved, ValueType: ValueNumber, Usage: "Only print the first N lines of subsequent files."},
	{Name: "tail", Type: CmdInterleaved, ValueType: ValueNumber, Usage: "Only print the last N lines of subsequent files."},
	{Name: "lines", Short: "n", Type: CmdInterleaved, ValueType: ValueNumber, Usage: "Limit output to N lines per file (head + tail). Use 0 for 'compact mode' (filenames and sizes only)."},
	{Name: "reset-lines", Short: "N", Type: CmdInterleaved, ValueType: ValueNone, Usage: "Revert to printing full file content for subsequent files."},

	// Filter flags
	{Name: "include", Short: "i", Type: CmdInterleaved, ValueType: ValueAny, Usage: "Add an include glob (OR logic). If present, only matching files are processed."},
	{Name: "exclude", Short: "e", Type: CmdInterleaved, ValueType: ValueAny, Usage: "Add an exclude glob. Takes precedence over includes."},
	{Name: "reset-filters", Short: "E", Type: CmdInterleaved, ValueType: ValueNone, Usage: "Clear all active include/exclude filters. Resumes processing all discovered files."},

	// Actions
	{Name: "file", Short: "f", Type: CmdAction, ValueType: ValueAny, Usage: "Explicitly add a file or directory path, bypassing global discovery depth limits."},
	{Name: "section", Short: "s", Type: CmdAction, ValueType: ValueAny, Usage: "Inject a section header and reset the [1/N] file progress counter."},
	{Name: "prompt", Short: "p", Type: CmdAction, ValueType: ValueAny, Usage: "Inject a custom text instruction directly into the output stream."},
}
