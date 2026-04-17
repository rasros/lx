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
		Category:  CatConfig,
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
		Long: `Enable line numbers for the next section.

This adds a numbered prefix (e.g., " 1: ") to every line of code. Extremely
useful when asking an LLM to reference exact locations in follow-up prompts.`,
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

Use 0 to enable compact mode: file content is skipped and only the filename
and size are shown.`,
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
		Name:      "documents",
		Short:     "D",
		Type:      CmdInterleaved,
		ValueType: ValueNone,
		Usage:     "Extract text from document files for subsequent paths",
		Long: `Extract plain text from document files for the next section.

By default, document files are treated as binary. When set, the following
formats are converted to plain text instead:
  - .pdf  - via PDF text extraction
  - .docx - via Word document parsing
  - .xlsx - via spreadsheet cell values (one sheet per section)
  - .pptx - via presentation text extraction`,
	},
	{
		Category:  CatInterleaved,
		Name:      "expand",
		Short:     "Z",
		Type:      CmdInterleaved,
		ValueType: ValueNone,
		Usage:     "Expand archive files inline for subsequent paths",
		Long: `Expand archive files inline for subsequent file arguments.

When set, archive files are opened and their contents processed as if they
were regular files in the directory tree. The archive path is used as a
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
		Category:  CatInterleaved,
		Name:      "hidden",
		Short:     "H",
		Type:      CmdInterleaved,
		ValueType: ValueNone,
		Usage:     "Include hidden files (dotfiles) for subsequent paths",
		Long: `Include hidden files and directories (starting with '.') for subsequent paths.

By default, dotfiles and dot-directories are excluded from directory walks.
This flag overrides that behaviour for the next section.`,
	},
	{
		Category:  CatInterleaved,
		Name:      "follow",
		Short:     "S",
		Type:      CmdInterleaved,
		ValueType: ValueNone,
		Usage:     "Follow directory symlinks for subsequent paths",
		Long: `Follow symbolic links that point to directories when walking.

By default, directory symlinks are skipped to avoid cycles. This flag enables
traversal into symlinked directories for the next section.`,
	},
	{
		Category:  CatInterleaved,
		Name:      "no-links",
		Type:      CmdInterleaved,
		ValueType: ValueNone,
		Usage:     "Skip symbolic links to files for subsequent paths",
		Long: `Skip symbolic links to files when walking directories.

By default, symlinks pointing to files are followed and their target content
included. This flag suppresses them for the next section.`,
	},
	{
		Category:  CatInterleaved,
		Name:      "no-ignore",
		Short:     "I",
		Type:      CmdInterleaved,
		ValueType: ValueNone,
		Usage:     "Disable .gitignore filtering for subsequent paths",
		Long: `Disable .gitignore-based file filtering for subsequent paths.

By default, lx respects .gitignore files (and the global git ignore file).
This flag disables that filtering so ignored files are included.`,
	},

	{
		Category:  CatInterleaved,
		Name:      "functions",
		Short:     "u",
		Type:      CmdInterleaved,
		ValueType: ValueNone,
		Usage:     "Show only function/method signatures for subsequent files",
		Long: `Filter files in the next section to show only function and method signatures.

Uses an AST-based parser to extract function definitions from source files.
The filtered lines are then subject to normal head/tail slicing.

Pairs with --types (-Y) to show a full code skeleton.`,
	},
	{
		Category:  CatInterleaved,
		Name:      "types",
		Short:     "Y",
		Type:      CmdInterleaved,
		ValueType: ValueNone,
		Usage:     "Show only type/class definitions for subsequent files",
		Long: `Filter files in the next section to show only type and class definitions,
including their field/variable declarations.

Uses an AST-based parser to extract type definitions from source files.
The filtered lines are then subject to normal head/tail slicing.

Pairs with --functions (-u) to show a full code skeleton.`,
	},

	{
		Category:  CatActions,
		Name:      "tree",
		Short:     "T",
		Type:      CmdAction,
		ValueType: ValueNone,
		Usage:     "Print file tree for the section (with content)",
		Long: `Print a file tree for all files in the current section, then also
include the file contents in the output. Use -t to suppress file content.

The tree uses the same walker settings (filters, ignore rules) as the file
listing, so it always matches the actual output.

The flag can appear before or after the files it applies to:
  lx -T src       (tree before src content)
  lx src -T       (tree after src content)
  lx . -T         (tree for the whole directory)

A "section" is a set of consecutive file arguments with no interleaved
options or section markers between them. Each section renders its own tree:
  lx -T src -s "Tests" test    (tree only for src, not test)
  lx -T src -i "*.go" lib      (two sections: tree for src, no tree for lib)
  lx -T src -f extra -- file2  (one section: tree covers src, extra, file2)`,
	},
	{
		Category:  CatActions,
		Name:      "tree-only",
		Short:     "t",
		Type:      CmdAction,
		ValueType: ValueNone,
		Usage:     "Print a file tree for the section (no content)",
		Long: `Print a file tree for all files in the current section without
including the file contents. This is useful for getting a structural overview.

Behaves identically to -T except file contents are suppressed:
  lx -t src       (show tree of src, no file content)
  lx -t .         (show tree of current directory)

See --tree (-T) for full details on section semantics.`,
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

This helps organize the output for the LLM by grouping related files. In
Markdown mode this renders as a "## Header" block; in XML mode it wraps
succeeding content in a named element.

-s also acts as a section boundary: all interleaved options (like -l, -n,
-i, -e) reset to their defaults for the files that follow.

Example:
  lx -s "Backend" -l src/ -s "Frontend" -i "*.ts" web/
The second -s resets -l so web/ gets full content, with only -i active.`,
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
