# lx configuration

lx is configured with YAML, and you only set the keys you care about. Anything you leave out keeps its built-in default,
so a three-line file is a perfectly normal config:

```yaml
output_mode: copy
verbosity: info
prompts_dir: ~/Workspaces/prompts
```

Two full profiles ship in the repo if you'd rather start from a complete file: 
[`default_config.yaml`](default_config.yaml) (Markdown, the runtime default, so you never have to load it) and 
[`xml_config.yaml`](xml_config.yaml) (XML, tuned for Claude).

## Loader chain

Config comes from several places, applied in order, with each one overriding only the keys it actually sets:

1. The user config at ~/.config/lx/config.yaml, loaded if present and ignored if not. Its path follows Go's
   os.UserConfigDir(), so it's $XDG_CONFIG_HOME/lx/config.yaml on Linux, ~/Library/Application Support/lx/config.yaml on
   macOS, and %AppData%\lx on Windows.
2. The environment variable LX_CONFIG, if set to a file path.
3. An explicit `lx -y path/to/file.yaml` (or `--config`), which errors if the path is missing.
4. Plain CLI flags like `--xml`, `--copy`, and `-v`, which always win.

## Built-in profiles

Both profiles are complete configs, so `-y default_config.yaml` behaves exactly like running with no `-y` at all. Reach
for one when you want a different format:

```bash
lx -y xml_config.yaml src/        # XML for one command
LX_CONFIG=$(pwd)/xml_config.yaml  # XML for the whole shell session
```

If you want to make changes stick, copy a profile into place and edit it; from then on plain lx picks it up, no -y flag
required:

```bash
cp default_config.yaml ~/.config/lx/config.yaml
```

## Top-level keys

All keys are optional. Templates not listed here aren't recognized.

| Key                       | Type     | Default        | Notes                                                                                             |
|---------------------------|----------|----------------|---------------------------------------------------------------------------------------------------|
| `output_format`           | enum     | `markdown`     | `markdown` \| `xml` \| `html` \| `bare`; any other value is an error. Overridden by `--md`/`--xml`/`--html`/`--bare`. |
| `output_mode`             | enum     | `stdout`       | `stdout` \| `copy`. Overridden by `-c`/`-C`/`-o`.                                                 |
| `show_stats`              | enum     | `auto`         | `auto` \| `always` \| `never`. Overridden by `--stats`/`--no-stats`/`-q`.                         |
| `verbosity`               | enum     | `warn`         | `debug` \| `info` \| `warn` \| `error` \| `silent` (or numeric `0`/`1`/`2+`). Overridden by `-v`. |
| `prompts_dir`             | string   | (unset)        | Library directory for `-P/--prompt-file`. Tilde-expanded.                                         |
| `prompt_extensions`       | string[] | see below      | Extensions probed when `-P <value>` doesn't include one. Default: `[".md", ".txt", ".prompt"]`.   |
| `file_content_template`   | template | format default | Renders one file's content. See Templates below.                                                  |
| `file_error_template`     | template | format default | Renders when a file can't be read.                                                                |
| `file_binary_template`    | template | format default | Renders when a file is detected as binary.                                                        |
| `file_compact_template`   | template | format default | Renders in compact mode (`-n 0`).                                                                 |
| `file_header_template`    | template | format default | Shared partial usable from the five other `file_*_templates` as `{{ template "file_header" . }}`.       |
| `file_image_template`     | template | format default | Renders an image file. Can call `file_content` and `file_binary` as well as `file_header`.        |
| `section_template`        | template | format default | Renders a logical section break (from `-s`).                                                      |
| `prompt_template`         | template | format default | Renders text injected via `-p` or `-P`.                                                           |
| `tree_template`           | template | format default | Renders the tree block from `-t`/`-T`.                                                            |
| `meta_template`           | template | format default | Renders the metadata block from `--system-context`.                                               |
| `section_header_template` | template | format default | Wraps the start of every section (XML uses this for `<content>`/`<section>`).                     |
| `section_footer_template` | template | format default | Wraps the end of every section.                                                                   |
| `output_header_template`  | template | format default | Rendered once before the first section.                                                           |
| `output_footer_template`  | template | format default | Rendered once after the last section.                                                             |
| `stats_template`          | template | built-in       | Renders the file/token summary footer.                                                            |
| `separators`              | bool     | format default | Blank line between rendered items, a newline between compact ones. See Format profiles.           |
| `embed_images`            | bool     | format default | Read images into `.DataURI` for `file_image_template`. See Format profiles.                       |

### Precedence for the prompts library

`-P/--prompt-file` looks for its library directory in this order, taking the first that exists:

1. `--prompts-dir <path>`
2. `$LX_PROMPTS_DIR`
3. `prompts_dir:` from the loaded config
4. `~/.config/lx/prompts`

The README's [Prompts and sections](README.md#prompts-and-sections) section covers how a name resolves to a file.

## Format profiles

`output_format` picks a profile: the default of every template, and of how output is laid out around them. A key you
set overrides that one property and leaves the rest of the profile in place, so `--html` with `separators: true` is
still HTML. The shipped YAML profiles spell out the Markdown and XML ones in full.

| Profile    | `separators` | `embed_images` | `file_image_template` renders an image as      |
|------------|--------------|----------------|------------------------------------------------|
| `markdown` | `true`       | `false`        | `file_binary`                                  |
| `xml`      | `true`       | `false`        | `file_binary`                                  |
| `html`     | `false`      | `true`         | an `<img>` of `.DataURI`, apart from content   |
| `bare`     | `false`      | `false`        | `file_binary`                                  |

With `separators`, lx writes a blank line between items (files, prompts, trees and section titles), a single newline
between two compact files, and a blank line before each section after the first. Without them items run together, so
the templates must end in the newlines they want, as HTML's do.

Every image renders with `file_image_template`. With `embed_images`, the image is read into `.DataURI` for the template
to inline, and `-M` leaves it an image rather than replacing it with its metadata. Without it `.DataURI` is empty, so
the profiles that do not embed render images as binaries. Turning it on for Markdown takes an image template that uses
the URI too:

```yaml
embed_images: true
file_image_template: |-
  {{ template "file_header" . }}
  ![{{ .Path }}]({{ .DataURI }})
```

## Templates

Templates are Go [`text/template`](https://pkg.go.dev/text/template). Each template key is handed a context value; the
fields it exposes and the helpers in scope are listed below.

### Contexts

All field names come from pkg/lx/core/types.go.

FileContext is bound to the six file templates (`file_content_template`, `file_error_template`, `file_binary_template`,
`file_compact_template`, `file_image_template`, and `file_header_template`):

| Field              | Type           | Notes                                              |
|--------------------|----------------|----------------------------------------------------|
| `Path`             | string         | Display path (relative when possible).             |
| `AbsPath`          | string         | Absolute path on disk, the URL fetched, or `stdin`. Empty for an archive entry. |
| `Archive`          | string         | For an archive entry, the archive's absolute path, or its URL when downloaded. |
| `Entry`            | string         | For an archive entry, its path inside the archive, with the directory a repository host wraps a repository's files in. The content of a single compressed file has the name shown for it: `notes.txt` for `notes.txt.gz`. |
| `Size`             | int64          | Bytes as rendered. Pair with `humanize`.           |
| `OriginalSize`     | int64          | Bytes on disk, before any conversion.              |
| `ModTime`          | time.Time      | Pair with `date "2006-01-02"`.                     |
| `TotalRows`        | int            | Lines in the whole file, however it is sliced.     |
| `IsEstimate`       | bool           | True if `TotalRows` is an estimate: only for a file over 64 MiB that is sliced or compact, so not read in full. |
| `Language`         | string         | Detected language hint for fenced code blocks.     |
| `Content`          | string         | Pair with `endNewline`.                            |
| `IsBinary`         | bool           |                                                    |
| `IsImage`          | bool           |                                                    |
| `DataURI`          | string         | An image's content as a `data:` URI, in the HTML format. |
| `IsCompactView`    | bool           |                                                    |
| `IsError`          | bool           |                                                    |
| `ReadError`        | string         | Set when `IsError`.                                |
| `TokenEstimate`    | int64          | Estimated tokens in `Content`, computed only when a template uses it. |
| `SkeletonMode`     | string         | `function signatures`, `type definitions`, `definitions`, or empty. |
| `ConvertedFrom`    | string         | Source format when content was converted, else empty. |
| `FileIndex`        | int            | 1-based index across the whole run.                |
| `SectionFileIndex` | int            | 1-based index within the current section.          |
| `Section`          | SectionContext | Outer section info (notably `Section.TotalFiles`). |
| `Global`           | Plan           | Whole-run info.                                    |

SectionContext is bound to `section_template`, `section_header_template`, and `section_footer_template`:

| Field        | Type          | Notes                                                          |
|--------------|---------------|----------------------------------------------------------------|
| `Body`       | string        | The section's title (from `-s "Title"`) or empty for implicit. |
| `Index`      | int           | 1-based section index.                                         |
| `TotalFiles` | int           |                                                                |
| `TotalSize`  | int64         |                                                                |
| `IsImplicit` | bool          | True for the auto-section before the first `-s`.               |
| `Global`     | Plan          |                                                                |

PromptContext is bound to `prompt_template`:

| Field     | Type           |
|-----------|----------------|
| `Body`    | string         |
| `Section` | SectionContext |
| `Global`  | Plan           |

TreeContext is bound to `tree_template` and has the same shape as PromptContext, except Body holds the rendered tree
text rather than an injected prompt string.

MetaContext is bound to `meta_template`. Body is the pre-rendered block, and Fields carries the same values
individually so a custom template can select and label them:

| Field     | Type              | Notes                                                        |
|-----------|-------------------|--------------------------------------------------------------|
| `Body`    | string            | The default rendered block.                                  |
| `Fields`  | map[string]string | Keys: `os`, `arch`, `time`, `host`, `version`, `command`.     |
| `Section` | SectionContext    |                                                              |
| `Global`  | Plan              |                                                              |

HeaderContext and FooterContext are bound to `output_header_template` and `output_footer_template`, and carry a single
field, Global (a Plan).

Plan is reachable from every other context as `.Global`. It is known before anything renders: TotalFiles, TotalSize,
SizeUnknown, TokenEstimate (estimated from file sizes), TotalSections, and SkippedBySize. Numbers only
rendering finds out, such as rows written or files that failed, exist only in `stats_template`. A template naming a
field its context lacks fails before any output is written, as a template that does not parse does.

StatsContext is bound to `stats_template`, which renders after the output is written. It carries Global, a Report, plus
ColorEnabled (a bool, false when output is piped or `NO_COLOR` is set). A Report has every Plan field and what only
rendering finds out: TotalWrittenBytes, TotalRows, BinaryFiles, FailedFiles, and WrittenTokenEstimate, estimated over
the written output where TokenEstimate is estimated from file sizes. The default template adds a third line listing
SkippedBySize, BinaryFiles, and FailedFiles, showing only the counters that are non-zero. Besides the helpers below,
`stats_template` can call `accent`, `dim` and `tokenLabel`, which color their text when ColorEnabled is set.

### Helper functions

Provided by `pkg/lx/templatex/defaults.go`:

| Helper       | Signature                       | Purpose                                                       |
|--------------|---------------------------------|---------------------------------------------------------------|
| `humanize`   | `int64 → string`                | `1024` → `"1.0 kB"`, etc.                                     |
| `endNewline` | `interface{} → string`          | Stringify and ensure exactly one trailing `\n`.               |
| `date`       | `layout, time.Time → string`    | Wrapper around `time.Format`.                                 |
| `dataURI`    | `path → string`                 | Reads a file on disk and emits a `data:<mime>;base64,…` URI. An empty path, the `AbsPath` of an archive entry, fails the template; use `.DataURI`. |
| `escape`     | `interface{} → string`          | HTML-escape (used by the HTML format's defaults).             |
| `commafy`    | `int → string`                  | `1234567` → `"1,234,567"`.                                    |
| `plural`     | `n, singular, plural → string`  | `singular` when n is 1, else `plural`.                        |

### Shared partials

Every `file_*_template` is parsed together with `file_header_template`, so any of them can call

```gotemplate
{{ template "file_header" . }}
```

to render the shared header. `file_image_template` can also call `file_content` and `file_binary`, which is how the
built-in profiles other than HTML render images. The Markdown profile uses this; the XML profile intentionally doesn't define
`file_header_template` (see the comment at the top of `xml_config.yaml`).

## Override recipes

To change one setting for good, put it in your user config and leave the rest alone. This makes every lx run copy to the
clipboard, with all templates still at their defaults:

```yaml
# ~/.config/lx/config.yaml
output_mode: copy
```

For a single project, a one-line file in the project root is enough. Setting `output_format` pulls in the XML *format
defaults*, so you don't need a copy of `xml_config.yaml` unless you want to tweak specific templates:

```bash
echo "output_format: xml" > .lx.yaml
lx -y .lx.yaml src/
```

Overriding a template works the same way, since only the keys you name are replaced:

```yaml
# ~/.config/lx/config.yaml
file_binary_template: |-
  {{ template "file_header" . }} (binary, {{ .Size | humanize }}, skipped)
```

And to point a whole shell session at a particular file, export `LX_CONFIG`; any single command can still override it
with `-y`:

```bash
export LX_CONFIG=$HOME/dotfiles/lx-claude.yaml
```
