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
	{Name: "config", Short: "y", Type: CmdGlobal, ValueType: ValueAny, Usage: "path to yaml config file"},
	{Name: "version", Short: "V", Type: CmdGlobal, ValueType: ValueNone, Usage: "print the version"},
	{Name: "help", Short: "h", Type: CmdGlobal, ValueType: ValueNone, Usage: "show help"},
	{Name: "quiet", Short: "q", Type: CmdGlobal, ValueType: ValueNone, Usage: "suppress debug output"},
	{Name: "verbose", Short: "v", Type: CmdGlobal, ValueType: ValueNone, Usage: "enable verbose output"},
	{Name: "cpuprofile", Type: CmdGlobal, ValueType: ValueAny, Usage: "write cpu profile to file", Internal: true},
	{Name: "memprofile", Type: CmdGlobal, ValueType: ValueAny, Usage: "write memory profile to file", Internal: true},

	{Name: "copy", Short: "c", Type: CmdGlobal, ValueType: ValueNone, Usage: "write output to clipboard"},
	{Name: "stdout", Short: "C", Type: CmdGlobal, ValueType: ValueNone, Usage: "write output to stdout"},
	{Name: "output", Short: "o", Type: CmdGlobal, ValueType: ValueAny, Usage: "write output to file"},

	{Name: "follow", Short: "L", Type: CmdGlobal, ValueType: ValueNone, Usage: "follow symbolic links"},
	{Name: "hidden", Short: "H", Type: CmdGlobal, ValueType: ValueNone, Usage: "search hidden files/directories"},
	{Name: "ignore", Type: CmdGlobal, ValueType: ValueNone, Usage: "respect ignore files"},
	{Name: "no-ignore", Short: "I", Type: CmdGlobal, ValueType: ValueNone, Usage: "disable ignore logic"},
	{Name: "no-follow", Type: CmdGlobal, ValueType: ValueNone, Usage: "do not follow symbolic links"},
	{Name: "no-hidden", Type: CmdGlobal, ValueType: ValueNone, Usage: "ignore hidden files"},

	{Name: "line-numbers", Short: "l", Type: CmdInterleaved, ValueType: ValueNone, Usage: "print line numbers"},
	{Name: "no-line-numbers", Short: "L", Type: CmdInterleaved, ValueType: ValueNone, Usage: "don't print line numbers"},
	{Name: "head", Type: CmdInterleaved, ValueType: ValueNumber, Usage: "print first N lines (0 = compact/skip)"},
	{Name: "tail", Type: CmdInterleaved, ValueType: ValueNumber, Usage: "print last N lines (0 = compact/skip)"},
	{Name: "lines", Short: "n", Type: CmdInterleaved, ValueType: ValueNumber, Usage: "print N lines split between head and tail"},
	{Name: "reset-lines", Short: "N", Type: CmdInterleaved, ValueType: ValueNone, Usage: "reset lines limits (and head/tail)"},

	{Name: "file", Short: "f", Type: CmdAction, ValueType: ValueAny, Usage: "explicit file path"},
	{Name: "section", Short: "s", Type: CmdAction, ValueType: ValueAny, Usage: "print a section header"},
	{Name: "prompt", Short: "p", Type: CmdAction, ValueType: ValueAny, Usage: "print custom text directly"},
}
