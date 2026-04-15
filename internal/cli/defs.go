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

This allows you to paste the context directly into an LLM interface without
dealing with temporary files or pipe redirection.

Supported system tools:
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
		Long: `Write the formatted result to a specific file path.

This is useful for generating large context bundles that you want to save
locally.`,
	},
	{
		Category:  CatFormatting,
		Name:      "md",
		Type:      CmdGlobal,
		ValueType: ValueNone,
		Usage:     "Format output as Markdown (default)",
		Long: `Format output as Markdown. This is the default output format.

It wraps file contents in fenced code blocks (\` + "```" + `language) and separates
files with horizontal rules. This format is widely understood by almost all
coding assistants (ChatGPT, DeepSeek, GitHub Copilot).`,
	},
	{
		Category:  CatFormatting,
		Name:      "xml",
		Type:      CmdGlobal,
		ValueType: ValueNone,
		Usage:     "Format output using XML tags (Claude optimized)",
		Long: `Format output using XML tags.

This format is highly recommended when working with Anthropic's Claude models.
It wraps file content in <document> and <source> tags. <section> tags are
especially recommended in xml mode.`,
	},
	{
		Category:  CatFormatting,
		Name:      "html",
		Type:      CmdGlobal,
		ValueType: ValueNone,
		Usage:     "Format output as a standalone HTML 5 file",
		Long: `Format output as a standalone HTML 5 file.

The output includes a responsive design (Pico CSS) and syntax highlighting
(highlight.js). This is ideal for creating shareable static archives of code
snippets, documentation, or logs that can be viewed in any browser.`,
	},
	{
		Category:  CatFormatting,
		Name:      "stdout",
		Short:     "C",
		Type:      CmdGlobal,
		ValueType: ValueNone,
		Usage:     "Write output to stdout (default)",
		Long: `Force output to stdout.

This is the default behavior unless --copy or --output is specified. You can
use this flag to override a "copy: true" setting in your configuration file
when you specifically want to pipe the output to another tool.`,
	},

	{
		Category:  CatDiscovery,
		Name:      "hidden",
		Short:     "H",
		Type:      CmdGlobal,
		ValueType: ValueNone,
		Usage:     "Include hidden files and directories",
		Long: `Include hidden files and directories (those starting with '.') in the traversal.

By default, lx ignores hidden files to avoid cluttering the context with git
internals (.git), IDE configurations (.vscode, .idea), or cache folders.
Use this flag if you need to include dotfiles like .env, .github, or .gitignore.`,
	},
	{
		Category:  CatDiscovery,
		Name:      "follow",
		Short:     "S",
		Type:      CmdGlobal,
		ValueType: ValueNone,
		Usage:     "Follow symbolic links to directories",
		Long: `Follow symbolic links that point to directories and traverse them recursively.

Be cautious with this flag in projects containing links to very large directories.`,
	},
	{
		Category:  CatDiscovery,
		Name:      "no-follow",
		Type:      CmdGlobal,
		ValueType: ValueNone,
		Usage:     "Do not follow directory symlinks (default)",
		Long: `Do not follow symbolic links that point to directories.

This is the default behavior to ensure predictable traversal within the
project root. This flag exists primarily to override a 'follow_symlinks: true'
setting in your configuration file.`,
	},
	{
		Category:  CatDiscovery,
		Name:      "links",
		Type:      CmdGlobal,
		ValueType: ValueNone,
		Usage:     "Include symbolic links to files (default)",
		Long: `Include symbolic links that point to files.

When encountered, lx will read the content of the target file. This is the
default behavior.`,
	},
	{
		Category:  CatDiscovery,
		Name:      "no-links",
		Type:      CmdGlobal,
		ValueType: ValueNone,
		Usage:     "Ignore symbolic links to files",
		Long: `Skip symbolic links that point to files.

Use this if your project contains many symlinks to large artifacts that you
do not want to include in the context.`,
	},
	{
		Category:  CatDiscovery,
		Name:      "ignore",
		Type:      CmdGlobal,
		ValueType: ValueNone,
		Usage:     "Respect ignore files (default)",
		Long: `Respect .gitignore, .ignore, and .lxignore files during traversal.

lx looks for ignore files in every directory it visits. The precedence order is:
  1. .lxignore (specific to this tool)
  2. .ignore (universal ignore file)
  3. .gitignore (git standard)
  4. Global ignore files (~/.config/lx/ignore or ~/.config/git/ignore)

Use this flag to override a previous --no-ignore setting.`,
	},
	{
		Category:  CatDiscovery,
		Name:      "no-ignore",
		Short:     "I",
		Type:      CmdGlobal,
		ValueType: ValueNone,
		Usage:     "Disregard all ignore files (traverse everything)",
		Long: `Disregard all ignore files and traverse everything.

When set, lx will NOT check .gitignore or any other ignore files. This is
useful when you explicitly need to include build artifacts, vendor directories,
or node_modules that are usually ignored.`,
	},
	{
		Category:  CatDiscovery,
		Name:      "no-hidden",
		Type:      CmdGlobal,
		ValueType: ValueNone,
		Usage:     "Skip hidden files (default)",
		Long: `Skip hidden files (files starting with '.').

This is the default behavior. Use this flag to override a 'show_hidden: true'
configuration setting.`,
	},
	{
		Category:  CatDiscovery,
		Name:      "expand",
		Short:     "Z",
		Type:      CmdGlobal,
		ValueType: ValueNone,
		Usage:     "Expand zip and other archive files inline",
		Long: `Expand archive files inline during traversal.

When enabled, archive files are opened and their contents are processed as if
they were regular files in the directory tree. The archive path is used as a
prefix for entries inside it (e.g. archive.zip/hello.txt).

Active include (-i) and exclude (-e) filters apply to entries inside the archive.
Hidden entry filtering (leading dot) is also respected.

Supported formats:
  ZIP-based:
    .zip, .jar, .war, .ear, .odt
  TAR (plain and compressed):
    .tar, .tar.gz, .tar.bz2, .tar.xz, .tar.zst,
    .tar.br, .tar.lz4, .tar.sz, .tar.s2,
    .tgz, .tbz2, .txz
  Other multi-file archives:
    .rar, .7z
  Single-file compression (exposed as one virtual entry):
    .gz, .bz2, .xz, .zst, .br, .lz4, .sz, .s2, .lz`,
	},
	{
		Category:  CatDiscovery,
		Name:      "no-expand",
		Type:      CmdGlobal,
		ValueType: ValueNone,
		Usage:     "Do not expand archive files (default)",
		Long: `Do not expand archive files; treat them as regular binary files.

This is the default behavior. Use this flag to override an
'expand_archives: true' setting in your configuration file.`,
	},
	{
		Category:  CatDiscovery,
		Name:      "documents",
		Type:      CmdGlobal,
		ValueType: ValueNone,
		Usage:     "Extract text from document files (default)",
		Long: `Extract plain text from document files during processing (default).

When enabled, document files are converted to plain text instead of being
treated as binary:
  - .pdf  — via PDF text extraction
  - .docx — via Word document parsing
  - .xlsx — via spreadsheet cell values (one sheet per section)

Note: .odt files are ZIP-based and are handled separately by the archive
expansion flag (--expand / -Z).

Use this flag to override a 'extract_documents: false' setting in your
configuration file.`,
	},
	{
		Category:  CatDiscovery,
		Name:      "no-documents",
		Short:     "D",
		Type:      CmdGlobal,
		ValueType: ValueNone,
		Usage:     "Do not extract document content (treat as binary)",
		Long: `Do not extract text from document files; treat them as binary.

Applies to .pdf, .docx, and .xlsx. Use this to override the default
'extract_documents: true' setting or a matching entry in your config file.`,
	},
	{
		Category:  CatDiscovery,
		Name:      "null",
		Short:     "0",
		Type:      CmdGlobal,
		ValueType: ValueNone,
		Usage:     "Expect NUL-terminated filenames from stdin",
		Long: `Expect NUL-terminated filenames when reading from stdin.

This is designed for pipeline compatibility with tools like 'find ... -print0'
or 'fd ... -0', which use the null character to safely handle filenames
containing spaces, newlines, or other special characters.`,
	},

	{
		Category:  CatInterleaved,
		Name:      "line-numbers",
		Short:     "l",
		Type:      CmdInterleaved,
		ValueType: ValueNone,
		Usage:     "Enable line numbers for subsequent files",
		Long: `Enable line numbers for all subsequent files in the stream.

This adds a numbered prefix (e.g., " 1: ") to every line of code. This is
extremely useful when asking an LLM to explain to reference exact locations 
in follow-up prompts.`,
	},
	{
		Category:  CatInterleaved,
		Name:      "reset-line-numbers",
		Short:     "L",
		Type:      CmdInterleaved,
		ValueType: ValueNone,
		Usage:     "Disable line numbers for subsequent files (default)",
		Long: `Disable line numbers for all subsequent files.

Use this to turn off line numbering for a specific set of files after having
enabled it previously with -l.`,
	},
	{
		Category:  CatInterleaved,
		Name:      "lines",
		Short:     "n",
		Type:      CmdInterleaved,
		ValueType: ValueNumber,
		Usage:     "Limit subsequent files to N lines (0 for compact)",
		Long: `Limit the content of subsequent files to N lines.

If a file is larger than N lines, lx will output the first N/2 lines and the
last N/2 lines, separated by a visual gap indicator.

Special Value:
  0: Enables 'compact' mode. The file content is skipped entirely, and only
     the filename and size are listed in the output. Useful for providing a
     file listing without consuming tokens.`,
	},
	{
		Category:  CatInterleaved,
		Name:      "reset-lines",
		Short:     "N",
		Type:      CmdInterleaved,
		ValueType: ValueNone,
		Usage:     "Reset slicing rules; print full content",
		Long: `Reset any active line limits (head, tail, or lines).

Subsequent files will be read and output in their entirety.`,
	},
	{
		Category:  CatInterleaved,
		Name:      "head",
		Type:      CmdInterleaved,
		ValueType: ValueNumber,
		Usage:     "Read only the first N lines of subsequent files",
		Long: `Read only the first N lines of subsequent files.

Useful for inspecting headers, imports, or configuration files where the
beginning is the most relevant part.`,
	},
	{
		Category:  CatInterleaved,
		Name:      "tail",
		Type:      CmdInterleaved,
		ValueType: ValueNumber,
		Usage:     "Read only the last N lines of subsequent files",
		Long: `Read only the last N lines of subsequent files.

Useful for log files, append-only journals, or checking the most recent 
additions to a file.`,
	},
	{
		Category:  CatInterleaved,
		Name:      "include",
		Short:     "i",
		Type:      CmdInterleaved,
		ValueType: ValueAny,
		Usage:     "Add a glob whitelist pattern to subsequent files",
		Long: `Add a glob whitelist pattern for subsequent file discovery.

If include patterns are provided, files MUST match at least one pattern to be
included in the output. This applies to directory traversal.

Examples:
  -i "*.go"       (Include only Go files)
  -i "src/**"     (Include files inside src)
  -i "*.{js,ts}"  (Include JS and TS files)`,
	},
	{
		Category:  CatInterleaved,
		Name:      "exclude",
		Short:     "e",
		Type:      CmdInterleaved,
		ValueType: ValueAny,
		Usage:     "Add a glob blacklist pattern to subsequent files",
		Long: `Add a glob blacklist pattern for subsequent file discovery.

Files matching ANY exclude pattern will be skipped, even if they match an
include pattern.

Examples:
  -e "*_test.go"  (Skip Go test files)
  -e "vendor/"    (Skip vendor directory)`,
	},
	{
		Category:  CatInterleaved,
		Name:      "reset-filters",
		Short:     "E",
		Type:      CmdInterleaved,
		ValueType: ValueNone,
		Usage:     "Clear all active include/exclude filters",
		Long: `Clear all active include (-i) and exclude (-e) filters.

Subsequent directory traversals will include all files (subject to standard
gitignore rules) until new filters are applied.`,
	},
	{
		Category:  CatInterleaved,
		Name:      "functions",
		Short:     "F",
		Type:      CmdInterleaved,
		ValueType: ValueNone,
		Usage:     "Show only function/method signatures for subsequent files",
		Long: `Filter subsequent files to show only function and method signatures.

Uses a simplified parser to extract function definitions from source files.
The filtered lines are then subject to normal head/tail slicing.

Supported languages: C, C++, Java, Python.

Pairs with --structs to show a full code skeleton.`,
	},
	{
		Category:  CatInterleaved,
		Name:      "no-functions",
		Type:      CmdInterleaved,
		ValueType: ValueNone,
		Usage:     "Disable function signature filtering for subsequent files",
		Long: `Stop filtering function signatures for subsequent files.

Use this to turn off function-only mode after having enabled it with -F.`,
	},
	{
		Category:  CatInterleaved,
		Name:      "structs",
		Short:     "T",
		Type:      CmdInterleaved,
		ValueType: ValueNone,
		Usage:     "Show only struct/class definitions for subsequent files",
		Long: `Filter subsequent files to show only struct and class definitions,
including their field/variable declarations.

Uses a simplified parser to extract type definitions from source files.
The filtered lines are then subject to normal head/tail slicing.

Supported languages: C, C++, Java, Python.

Pairs with --functions to show a full code skeleton.`,
	},
	{
		Category:  CatInterleaved,
		Name:      "no-structs",
		Type:      CmdInterleaved,
		ValueType: ValueNone,
		Usage:     "Disable struct/class filtering for subsequent files",
		Long: `Stop filtering struct and class definitions for subsequent files.

Use this to turn off struct-only mode after having enabled it with -T.`,
	},
	{
		Category:  CatInterleaved,
		Name:      "reset-skeleton",
		Type:      CmdInterleaved,
		ValueType: ValueNone,
		Usage:     "Disable all skeleton filters (functions and structs)",
		Long: `Disable all skeleton extraction filters for subsequent files.

Subsequent files will be shown in full, as if --functions and --structs
had never been set.`,
	},

	{
		Category:  CatActions,
		Name:      "file",
		Short:     "f",
		Type:      CmdAction,
		ValueType: ValueAny,
		Usage:     "Force process a specific path (bypasses ignores/filters)",
		Long: `Force process a specific file path immediately.

Unlike passing a path as a standard argument, using -f forces the file to be
included even if it is:
  - Listed in .gitignore
  - Hidden (starts with .)
  - Excluded by active -e filters
  - Not included by active -i filters

If the path is a directory, active filters (-i/-e) are preserved for the
recursive walk, but hidden/.gitignore checks are still disabled.`,
	},
	{
		Category:  CatActions,
		Name:      "section",
		Short:     "s",
		Type:      CmdAction,
		ValueType: ValueAny,
		Usage:     "Insert a logical separator with a header",
		Long: `Insert a logical section separator with a custom header.

This helps organize the output for the LLM. For example, you can group all
backend files under one section and frontend files under another.
In Markdown mode, this renders as a "## Header" block. In XML mode this wraps
succeeding content.
`,
	},
	{
		Category:  CatActions,
		Name:      "prompt",
		Short:     "p",
		Type:      CmdAction,
		ValueType: ValueAny,
		Usage:     "Inject a custom text prompt into the output",
		Long: `Inject a custom text block directly into the output stream.

Useful for adding specific instructions to the LLM alongside the code context.
Example:
  lx -p "Refactor the following code for better concurrency:" main.go
`,
	},

	{
		Category:  CatConfig,
		Name:      "config",
		Short:     "y",
		Type:      CmdGlobal,
		ValueType: ValueAny,
		Usage:     "Load configuration from a specific YAML file",
		Long: `Load configuration from a specific YAML file.

Overrides default config paths (~/.config/lx/config.yaml or $LX_CONFIG).
Configuration priorities are:
  1. Command line flags
  2. Specific config file (-y)
  3. Environment variable ($LX_CONFIG)
  4. User config directory`,
	},
	{
		Category:  CatConfig,
		Name:      "quiet",
		Short:     "q",
		Type:      CmdGlobal,
		ValueType: ValueNone,
		Usage:     "Suppress the stats summary and logging",
		Long: `Suppress the final statistics summary and all non-error logging.

Useful for clean piping where you don't want metadata on stderr interfering
with scripts, though lx writes content to stdout and logs/stats to stderr by
default anyway.`,
	},
	{
		Category:  CatConfig,
		Name:      "verbose",
		Short:     "v",
		Type:      CmdGlobal,
		ValueType: ValueOptional,
		Usage:     "Set log level [possible values: debug, info, warn, error, silent]",
		Long: `Set the internal logging verbosity.

Usage:
  -v, --verbose        Sets level to Info
  -vv                  Sets level to Debug
  --verbose=debug      Sets level to Debug
  --verbose=info       Sets level to Info
  --verbose=warn       Sets level to Warn
  --verbose=error      Sets level to Error
  --verbose=silent     Disables all logging`,
	},
	{
		Category:  CatConfig,
		Name:      "stats",
		Type:      CmdGlobal,
		ValueType: ValueNone,
		Usage:     "Force display of file/token summary to stderr",
		Long: `Force the display of the file count, size, and token summary to stderr.

By default, statistics are hidden if output is being piped to another process
(unless writing to a file or clipboard). This flag forces them to appear.`,
	},
	{
		Category:  CatConfig,
		Name:      "no-stats",
		Type:      CmdGlobal,
		ValueType: ValueNone,
		Usage:     "Disable file/token summary footer entirely",
		Long: `Disable the file count, size, and token summary footer entirely.

Use this if you want the process to finish silently without printing metadata
to the terminal.`,
	},
	{
		Category:  CatConfig,
		Name:      "version",
		Short:     "V",
		Type:      CmdGlobal,
		ValueType: ValueNone,
		Usage:     "Print version information and exit",
		Long:      "Print the current version information and exit.",
	},
	{
		Category:  CatConfig,
		Name:      "help",
		Short:     "h",
		Type:      CmdGlobal,
		ValueType: ValueNone,
		Usage:     "Show help (use --help for detailed documentation)",
		Long:      "Show the help message. Use --help for the detailed manual style help.",
	},

	{Name: "cpuprofile", Type: CmdGlobal, ValueType: ValueAny, Usage: "Write CPU profile to file.", Internal: true},
	{Name: "memprofile", Type: CmdGlobal, ValueType: ValueAny, Usage: "Write memory profile to file.", Internal: true},
}
