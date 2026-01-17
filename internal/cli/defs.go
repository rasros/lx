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

const (
	CatFormatting  = "Formatting"
	CatDiscovery   = "Discovery"
	CatInterleaved = "Interleaved"
	CatActions     = "Actions"
	CatConfig      = "Config"
)

var definitions = []CommandDef{
	{
		Category:  CatFormatting,
		Name:      "copy",
		Short:     "c",
		Type:      CmdGlobal,
		ValueType: ValueNone,
		Usage:     "Copy the final output to the system clipboard",
		Long: `Copy the final formatted output directly to the system clipboard.

This feature depends on system tools:
  - macOS: pbcopy
  - Linux: xclip (X11) or wl-copy (Wayland)
  - Windows: clip`,
	},
	{
		Category:  CatFormatting,
		Name:      "output",
		Short:     "o",
		Type:      CmdGlobal,
		ValueType: ValueAny,
		Usage:     "Write the result to a file path instead of stdout",
		Long: `Write the formatted result to a specific file path instead of stdout.
This allows you to save the context directly to a file while still seeing
stats in your terminal.`,
	},
	{
		Category:  CatFormatting,
		Name:      "md",
		Type:      CmdGlobal,
		ValueType: ValueNone,
		Usage:     "Format output as Markdown (default)",
		Long: `Format output as Markdown (default).

This is the standard format, ideal for most coding assistants like ChatGPT,
DeepSeek, or GitHub Copilot. It uses fenced code blocks with language identifiers.`,
	},
	{
		Category:  CatFormatting,
		Name:      "xml",
		Type:      CmdGlobal,
		ValueType: ValueNone,
		Usage:     "Format output using XML tags (Claude optimized)",
		Long: `Format output using XML tags.

Recommended for Anthropic's Claude. XML tags help the model clearly distinguish
between file contents, metadata, and user instructions, especially in large contexts.`,
	},
	{
		Category:  CatFormatting,
		Name:      "html",
		Type:      CmdGlobal,
		ValueType: ValueNone,
		Usage:     "Format output as a standalone HTML 5 file",
		Long: `Format output as a standalone HTML 5 file.

The output includes Pico CSS for styling and syntax highlighting. This is useful
for creating shareable archives or viewing code in a browser.`,
	},
	{
		Category:  CatFormatting,
		Name:      "stdout",
		Short:     "C",
		Type:      CmdGlobal,
		ValueType: ValueNone,
		Usage:     "Write output to stdout (default)",
		Long: `Force output to stdout, overriding any settings in the config file.
Useful if your config defaults to "copy" mode but you want to pipe the output.`,
	},

	{
		Category:  CatDiscovery,
		Name:      "hidden",
		Short:     "H",
		Type:      CmdGlobal,
		ValueType: ValueNone,
		Usage:     "Include hidden files and directories",
		Long: `Include hidden files and directories (starting with '.') in the traversal.
        
By default, lx skips hidden files to avoid cluttering the context with git
internals or config caches.`,
	},
	{
		Category:  CatDiscovery,
		Name:      "follow",
		Short:     "S",
		Type:      CmdGlobal,
		ValueType: ValueNone,
		Usage:     "Follow symbolic links to directories",
		Long:      `Follow symbolic links that point to directories and traverse them recursively.`,
	},
	{
		Category:  CatDiscovery,
		Name:      "no-follow",
		Type:      CmdGlobal,
		ValueType: ValueNone,
		Usage:     "Do not follow directory symlinks (default)",
		Long:      `Do not follow symbolic links that point to directories. This overrides any 'follow_symlinks: true' setting in the config file.`,
	},
	{
		Category:  CatDiscovery,
		Name:      "links",
		Type:      CmdGlobal,
		ValueType: ValueNone,
		Usage:     "Include symbolic links to files (default)",
		Long:      `Include symbolic links that point to files. This overrides any 'no_file_links: true' setting in the config file.`,
	},
	{
		Category:  CatDiscovery,
		Name:      "no-links",
		Type:      CmdGlobal,
		ValueType: ValueNone,
		Usage:     "Ignore symbolic links to files",
		Long:      `Ignore symbolic links that point to files. By default, file symlinks are included and their targets read.`,
	},
	{
		Category:  CatDiscovery,
		Name:      "ignore",
		Type:      CmdGlobal,
		ValueType: ValueNone,
		Usage:     "Respect ignore files (default)",
		Long: `Respect .gitignore, .ignore, and .lxignore files.
        
The precedence order for ignoring files is:
1. .lxignore (in directory)
2. .ignore (in directory)
3. .gitignore (in directory)
4. Global ignore files (~/.config/lx/ignore or ~/.config/git/ignore)

Use this flag only if you need to override a previous --no-ignore flag.`,
	},
	{
		Category:  CatDiscovery,
		Name:      "no-ignore",
		Short:     "I",
		Type:      CmdGlobal,
		ValueType: ValueNone,
		Usage:     "Disregard all ignore files (traverse everything)",
		Long: `Disregard all ignore files and traverse everything.
        
When this flag is set, lx will NOT check .gitignore, .ignore, .lxignore, or
any global ignore files. 

This is useful when you explicitly want to include build artifacts, vendor
directories, or other usually ignored content.`,
	},
	{
		Category:  CatDiscovery,
		Name:      "no-hidden",
		Type:      CmdGlobal,
		ValueType: ValueNone,
		Usage:     "Skip hidden files (default)",
		Long:      "Skip hidden files (files starting with '.'). This is the default behavior.",
	},
	{
		Category:  CatDiscovery,
		Name:      "null",
		Short:     "0",
		Type:      CmdGlobal,
		ValueType: ValueNone,
		Usage:     "Expect NUL-terminated filenames from stdin",
		Long: `Expect NUL-terminated filenames from stdin.
        
Useful for handling filenames with spaces or newlines when piping from tools
like 'find -print0' or 'fd -0'.`,
	},

	{
		Category:  CatInterleaved,
		Name:      "line-numbers",
		Short:     "l",
		Type:      CmdInterleaved,
		ValueType: ValueNone,
		Usage:     "Enable line numbers for subsequent files",
		Long: `Enable line numbers for subsequent files.
        
Useful when you want to ask the LLM about specific lines of code. This setting
persists until reset or disabled.`,
	},
	{
		Category:  CatInterleaved,
		Name:      "reset-line-numbers",
		Short:     "L",
		Type:      CmdInterleaved,
		ValueType: ValueNone,
		Usage:     "Disable line numbers for subsequent files (default)",
		Long:      "Disable line numbers for subsequent files.",
	},
	{
		Category:  CatInterleaved,
		Name:      "lines",
		Short:     "n",
		Type:      CmdInterleaved,
		ValueType: ValueNumber,
		Usage:     "Limit subsequent files to N lines (0 for compact)",
		Long: `Limit subsequent files to N lines.
        
If the file is larger than N lines, lx will output the first N/2 lines and
the last N/2 lines, separated by a gap indicator.

Set to 0 for 'compact' mode, which only prints file names and sizes.`,
	},
	{
		Category:  CatInterleaved,
		Name:      "reset-lines",
		Short:     "N",
		Type:      CmdInterleaved,
		ValueType: ValueNone,
		Usage:     "Reset slicing rules; print full content",
		Long:      "Reset slicing rules; print full content for subsequent files.",
	},
	{
		Category:  CatInterleaved,
		Name:      "head",
		Type:      CmdInterleaved,
		ValueType: ValueNumber,
		Usage:     "Read only the first N lines of subsequent files",
		Long:      "Read only the first N lines of subsequent files.",
	},
	{
		Category:  CatInterleaved,
		Name:      "tail",
		Type:      CmdInterleaved,
		ValueType: ValueNumber,
		Usage:     "Read only the last N lines of subsequent files",
		Long:      "Read only the last N lines of subsequent files.",
	},
	{
		Category:  CatInterleaved,
		Name:      "include",
		Short:     "i",
		Type:      CmdInterleaved,
		ValueType: ValueAny,
		Usage:     "Add a glob whitelist pattern to subsequent files",
		Long: `Add a glob whitelist pattern.
        
Only files matching this pattern will be included. You can specify multiple
includes. If includes are present, files must match at least one include pattern.

Example: -i "*.go" -i "*.js"`,
	},
	{
		Category:  CatInterleaved,
		Name:      "exclude",
		Short:     "e",
		Type:      CmdInterleaved,
		ValueType: ValueAny,
		Usage:     "Add a glob blacklist pattern to subsequent files",
		Long: `Add a glob blacklist pattern.
        
Files matching this pattern will be skipped, even if they match an include pattern.

Example: -e "*_test.go"`,
	},
	{
		Category:  CatInterleaved,
		Name:      "reset-filters",
		Short:     "E",
		Type:      CmdInterleaved,
		ValueType: ValueNone,
		Usage:     "Clear all active include/exclude filters",
		Long:      "Clear all active include/exclude filters. Subsequent files will not be filtered.",
	},

	{
		Category:  CatActions,
		Name:      "file",
		Short:     "f",
		Type:      CmdAction,
		ValueType: ValueAny,
		Usage:     "Process a specific path bypassing ignores",
		Long: `Process a specific path immediately.
        
Unlike passing a path as a raw argument, this action bypasses gitignore checks
and applies current interleaving settings immediately.`,
	},
	{
		Category:  CatActions,
		Name:      "section",
		Short:     "s",
		Type:      CmdAction,
		ValueType: ValueAny,
		Usage:     "Insert a logical separator with a header",
		Long: `Insert a logical separator with a header.
        
This helps organize the output into distinct sections, which is useful when
grouping related files for the LLM.`,
	},
	{
		Category:  CatActions,
		Name:      "prompt",
		Short:     "p",
		Type:      CmdAction,
		ValueType: ValueAny,
		Usage:     "Inject a custom text prompt into the output",
		Long: `Inject a custom text prompt or instruction directly into the output stream.
        
The value is treated as raw text.`,
	},

	{
		Category:  CatConfig,
		Name:      "config",
		Short:     "y",
		Type:      CmdGlobal,
		ValueType: ValueAny,
		Usage:     "Load configuration from a specific YAML file",
		Long:      "Load configuration from a specific YAML file.",
	},
	{
		Category:  CatConfig,
		Name:      "quiet",
		Short:     "q",
		Type:      CmdGlobal,
		ValueType: ValueNone,
		Usage:     "Suppress the stats summary and logging",
		Long:      "Suppress the stats summary and logging",
	},
	{
		Category:  CatConfig,
		Name:      "verbose",
		Short:     "v",
		Type:      CmdGlobal,
		ValueType: ValueOptional,
		Usage:     "Set log level [possible values: info, debug, silent]",
		Long: `Set log level.
        
Use -v for info, -vv for debug. Alternatively, provide an explicit level:
--verbose=debug, --verbose=error`,
	},
	{
		Category:  CatConfig,
		Name:      "stats",
		Type:      CmdGlobal,
		ValueType: ValueNone,
		Usage:     "Force display of file/token summary to stderr",
		Long:      "Force display of file/token summary to stderr, even if output is not redirected.",
	},
	{
		Category:  CatConfig,
		Name:      "no-stats",
		Type:      CmdGlobal,
		ValueType: ValueNone,
		Usage:     "Disable file/token summary footer entirely",
		Long:      "Disable file/token summary footer entirely.",
	},
	{
		Category:  CatConfig,
		Name:      "version",
		Short:     "V",
		Type:      CmdGlobal,
		ValueType: ValueNone,
		Usage:     "Print version information and exit",
		Long:      "Print version information and exit.",
	},
	{
		Category:  CatConfig,
		Name:      "help",
		Short:     "h",
		Type:      CmdGlobal,
		ValueType: ValueNone,
		Usage:     "Show help (use --help for detailed documentation)",
		Long:      "Show help. Use --help for detailed usage information.",
	},

	{Name: "cpuprofile", Type: CmdGlobal, ValueType: ValueAny, Usage: "Write CPU profile to file.", Internal: true},
	{Name: "memprofile", Type: CmdGlobal, ValueType: ValueAny, Usage: "Write memory profile to file.", Internal: true},
}
