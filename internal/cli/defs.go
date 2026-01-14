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
	{Name: "config", Short: "y", Type: CmdGlobal, ValueType: ValueAny, Usage: "path to yaml config file"},
	{Name: "version", Short: "V", Type: CmdGlobal, ValueType: ValueNone, Usage: "print the version"},
	{Name: "help", Short: "h", Type: CmdGlobal, ValueType: ValueNone, Usage: "show help"},

	// Logging
	{Name: "quiet", Short: "q", Type: CmdGlobal, ValueType: ValueNone, Usage: "silence all log output"},
	// explicit --verbose level
	{Name: "verbose", Type: CmdGlobal, ValueType: ValueAny, Usage: "log level (trace, debug, info, warn, error)"},
	// short -v counter
	{Name: "verbosity", Short: "v", Type: CmdGlobal, ValueType: ValueCounter, Usage: "increase verbosity (-v = info, -vv = debug, -vvv trace)"},

	// Stats Control
	{Name: "stats", Type: CmdGlobal, ValueType: ValueNone, Usage: "force show summary stats"},
	{Name: "no-stats", Type: CmdGlobal, ValueType: ValueNone, Usage: "disable summary stats"},

	{Name: "null", Short: "0", Type: CmdGlobal, ValueType: ValueNone, Usage: "expect NUL-terminated filenames from stdin"},

	// Format flags
	{Name: "md", Type: CmdGlobal, ValueType: ValueNone, Usage: "use markdown output format"},
	{Name: "xml", Type: CmdGlobal, ValueType: ValueNone, Usage: "use xml output format"},
	{Name: "html", Type: CmdGlobal, ValueType: ValueNone, Usage: "use html output format"},

	// Profiling
	{Name: "cpuprofile", Type: CmdGlobal, ValueType: ValueAny, Usage: "write cpu profile to file", Internal: true},
	{Name: "memprofile", Type: CmdGlobal, ValueType: ValueAny, Usage: "write memory profile to file", Internal: true},

	// Output format
	{Name: "copy", Short: "c", Type: CmdGlobal, ValueType: ValueNone, Usage: "write output to clipboard"},
	{Name: "stdout", Short: "C", Type: CmdGlobal, ValueType: ValueNone, Usage: "write output to stdout"},
	{Name: "output", Short: "o", Type: CmdGlobal, ValueType: ValueAny, Usage: "write output to file"},

	// Global flags
	{Name: "follow", Short: "L", Type: CmdGlobal, ValueType: ValueNone, Usage: "follow symbolic links"},
	{Name: "hidden", Short: "H", Type: CmdGlobal, ValueType: ValueNone, Usage: "search hidden files/directories"},
	{Name: "ignore", Type: CmdGlobal, ValueType: ValueNone, Usage: "respect ignore files"},
	{Name: "no-ignore", Short: "I", Type: CmdGlobal, ValueType: ValueNone, Usage: "disable ignore logic"},
	{Name: "no-follow", Type: CmdGlobal, ValueType: ValueNone, Usage: "do not follow symbolic links"},
	{Name: "no-hidden", Type: CmdGlobal, ValueType: ValueNone, Usage: "ignore hidden files"},

	// Interleaved flags
	{Name: "line-numbers", Short: "l", Type: CmdInterleaved, ValueType: ValueNone, Usage: "print line numbers"},
	{Name: "no-line-numbers", Short: "L", Type: CmdInterleaved, ValueType: ValueNone, Usage: "don't print line numbers"},
	{Name: "head", Type: CmdInterleaved, ValueType: ValueNumber, Usage: "print first N lines (0 = compact/skip)"},
	{Name: "tail", Type: CmdInterleaved, ValueType: ValueNumber, Usage: "print last N lines (0 = compact/skip)"},
	{Name: "lines", Short: "n", Type: CmdInterleaved, ValueType: ValueNumber, Usage: "print N lines split between head and tail"},
	{Name: "reset-lines", Short: "N", Type: CmdInterleaved, ValueType: ValueNone, Usage: "reset lines limits (and head/tail)"},

	// Filter flags
	{Name: "include", Short: "i", Type: CmdInterleaved, ValueType: ValueAny, Usage: "include only files matching glob"},
	{Name: "exclude", Short: "e", Type: CmdInterleaved, ValueType: ValueAny, Usage: "exclude files matching glob"},
	{Name: "reset-filters", Short: "E", Type: CmdInterleaved, ValueType: ValueNone, Usage: "reset include/exclude filters"},

	// Actions
	{Name: "file", Short: "f", Type: CmdAction, ValueType: ValueAny, Usage: "explicit file path"},
	{Name: "section", Short: "s", Type: CmdAction, ValueType: ValueAny, Usage: "print a section header"},
	{Name: "prompt", Short: "p", Type: CmdAction, ValueType: ValueAny, Usage: "print custom text directly"},
}
