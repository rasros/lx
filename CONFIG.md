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
| `output_format`           | enum     | `markdown`     | `markdown` \| `xml` \| `html` \| `bare`. Overridden by `--md`/`--xml`/`--html`/`--bare`.          |
| `output_mode`             | enum     | `stdout`       | `stdout` \| `copy`. Overridden by `-c`/`-C`/`-o`.                                                 |
| `show_stats`              | enum     | `auto`         | `auto` \| `always` \| `never`. Overridden by `--stats`/`--no-stats`/`-q`.                         |
| `verbosity`               | enum     | `warn`         | `debug` \| `info` \| `warn` \| `error` \| `silent` (or numeric `0`/`1`/`2+`). Overridden by `-v`. |
| `prompts_dir`             | string   | (unset)        | Library directory for `-P/--prompt-file`. Tilde-expanded.                                         |
| `prompt_extensions`       | string[] | see below      | Extensions probed when `-P <value>` doesn't include one. Default: `[".md", ".txt", ".prompt"]`.   |
| `file_content_template`   | template | format default | Renders one file's content. See Templates below.                                                  |
| `file_error_template`     | template | format default | Renders when a file can't be read.                                                                |
| `file_binary_template`    | template | format default | Renders when a file is detected as binary.                                                        |
| `file_compact_template`   | template | format default | Renders in compact mode (`-n 0`).                                                                 |
| `file_header_template`    | template | format default | Shared partial usable from the four `file_*_templates` as `{{ template "file_header" . }}`.       |
| `section_template`        | template | format default | Renders a logical section break (from `-s`).                                                      |
| `prompt_template`         | template | format default | Renders text injected via `-p` or `-P`.                                                           |
| `tree_template`           | template | format default | Renders the tree block from `-t`/`-T`.                                                            |
| `section_header_template` | template | format default | Wraps the start of every section (XML uses this for `<content>`/`<section>`).                     |
| `section_footer_template` | template | format default | Wraps the end of every section.                                                                   |
| `output_header_template`  | template | format default | Rendered once before the first section.                                                           |
| `output_footer_template`  | template | format default | Rendered once after the last section.                                                             |
| `stats_template`          | template | format default | Renders the file/token summary footer.                                                            |

### Precedence for the prompts library

`-P/--prompt-file` looks for its library directory in this order, taking the first that exists:

1. `--prompts-dir <path>`
2. `$LX_PROMPTS_DIR`
3. `prompts_dir:` from the loaded config
4. `~/.config/lx/prompts`

The README's [Prompts and sections](README.md#prompts-and-sections) section covers how a name resolves to a file.

## Templates

Templates are Go [`text/template`](https://pkg.go.dev/text/template). Each template key is handed a context value; the
fields it exposes and the helpers in scope are listed below.

### Contexts

All field names come from pkg/lx/core/types.go.

FileContext is bound to the five file templates (`file_content_template`, `file_error_template`, `file_binary_template`,
`file_compact_template`, and `file_header_template`):

| Field              | Type           | Notes                                              |
|--------------------|----------------|----------------------------------------------------|
| `Path`             | string         | Display path (relative when possible).             |
| `AbsPath`          | string         | Absolute path on disk.                             |
| `Size`             | int64          | Bytes. Pair with `humanize`.                       |
| `ModTime`          | time.Time      | Pair with `date "2006-01-02"`.                     |
| `TotalRows`        | int            | Lines after slicing.                               |
| `IsEstimate`       | bool           | True if rows are an estimate (e.g. binary skip).   |
| `Language`         | string         | Detected language hint for fenced code blocks.     |
| `Content`          | interface{}    | Pair with `endNewline`.                            |
| `IsBinary`         | bool           |                                                    |
| `IsImage`          | bool           |                                                    |
| `IsCompactView`    | bool           |                                                    |
| `IsError`          | bool           |                                                    |
| `ReadError`        | string         | Set when `IsError`.                                |
| `TokenEstimate`    | int64          |                                                    |
| `SkeletonMode`     | string         | `functions`, `types`, or empty.                    |
| `FileIndex`        | int            | 1-based index across the whole run.                |
| `SectionFileIndex` | int            | 1-based index within the current section.          |
| `Section`          | SectionContext | Outer section info (notably `Section.TotalFiles`). |
| `Global`           | GlobalContext  | Whole-run info.                                    |

SectionContext is bound to `section_template`, `section_header_template`, and `section_footer_template`:

| Field        | Type          | Notes                                                          |
|--------------|---------------|----------------------------------------------------------------|
| `Body`       | string        | The section's title (from `-s "Title"`) or empty for implicit. |
| `Index`      | int           | 1-based section index.                                         |
| `TotalFiles` | int           |                                                                |
| `TotalSize`  | int64         |                                                                |
| `IsImplicit` | bool          | True for the auto-section before the first `-s`.               |
| `Global`     | GlobalContext |                                                                |

PromptContext is bound to `prompt_template`:

| Field     | Type           |
|-----------|----------------|
| `Body`    | string         |
| `Section` | SectionContext |
| `Global`  | GlobalContext  |

TreeContext is bound to `tree_template` and has the same shape as PromptContext, except Body holds the rendered tree
text rather than an injected prompt string.

HeaderContext and FooterContext are bound to `output_header_template` and `output_footer_template`, and carry a single
field, Global (a GlobalContext).

StatsContext is bound to `stats_template` and carries Global only.

GlobalContext is reachable from every other context as `.Global`. Its fields are TotalFiles, TotalSize,
TotalWrittenBytes, TokenEstimate, TotalSections, WorkDir, and Metadata (a map of string to string).

### Helper functions

Provided by `pkg/lx/templatex/defaults.go`:

| Helper       | Signature                       | Purpose                                                       |
|--------------|---------------------------------|---------------------------------------------------------------|
| `humanize`   | `int64 → string`                | `1024` → `"1.0 kB"`, etc.                                     |
| `endNewline` | `interface{} → string`          | Stringify and ensure exactly one trailing `\n`.               |
| `date`       | `layout, time.Time → string`    | Wrapper around `time.Format`.                                 |
| `dataURI`    | `path → string`                 | Reads a file and emits a `data:<mime>;base64,…` URI.          |
| `escape`     | `interface{} → string`          | HTML-escape (used by the HTML format's defaults).             |

### Shared partials

The four `file_*_templates` and the `file_header_template` are parsed into a single template set, so any of them can
call

```gotemplate
{{ template "file_header" . }}
```

to render the shared header. The Markdown profile uses this; the XML profile intentionally doesn't define
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
