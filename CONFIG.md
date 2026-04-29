# lx configuration

`lx` is configured via YAML. Two complete profiles ship in the repo:

- [`default_config.yaml`](default_config.yaml) — Markdown output (the runtime
  default; you don't need to load it explicitly).
- [`xml_config.yaml`](xml_config.yaml) — XML output, optimized for Claude.

You only ever need to set the keys you want to change — everything you omit
keeps its built-in default.

## Loader chain

Sources are applied in order; later sources override only the keys they set.

1. **User config**: `~/.config/lx/config.yaml` (resolved via Go's
   `os.UserConfigDir()` — `$XDG_CONFIG_HOME/lx/config.yaml` on Linux,
   `~/Library/Application Support/lx/config.yaml` on macOS, `%AppData%\lx`
   on Windows). Loaded if present, ignored if not.
2. **Environment**: `$LX_CONFIG=/path/to/file.yaml` if set.
3. **Explicit flag**: `lx -y /path/to/file.yaml …` (or `--config`). Errors
   if the path is missing.
4. **CLI flags**: `--xml`, `--copy`, `-v`, `--prompts-dir`, etc., always win.

This means you can drop a 3-line `~/.config/lx/config.yaml` like

```yaml
output_mode: copy
verbosity: info
prompts_dir: ~/Workspaces/prompts
```

and the rest of the defaults stay intact.

## Built-in profiles

Both profiles are full configs; running `-y default_config.yaml` is identical
to running with no `-y` at all. To switch formats per-invocation:

```bash
lx -y xml_config.yaml src/        # XML for one command
LX_CONFIG=$(pwd)/xml_config.yaml  # XML for the whole shell session
```

To customize, copy a profile and edit only what you need:

```bash
cp default_config.yaml ~/.config/lx/config.yaml
# edit, then run lx normally — no -y needed
```

## Top-level keys

All keys are optional. Templates not listed here aren't recognized.

| Key                       | Type      | Default        | Notes                                                                                              |
| ------------------------- | --------- | -------------- | -------------------------------------------------------------------------------------------------- |
| `output_format`           | enum      | `markdown`     | `markdown` \| `xml` \| `html` \| `bare`. Overridden by `--md`/`--xml`/`--html`/`--bare`.           |
| `output_mode`             | enum      | `stdout`       | `stdout` \| `copy`. Overridden by `-c`/`-C`/`-o`.                                                  |
| `show_stats`              | enum      | `auto`         | `auto` \| `always` \| `never`. Overridden by `--stats`/`--no-stats`/`-q`.                          |
| `verbosity`               | enum      | `warn`         | `debug` \| `info` \| `warn` \| `error` \| `silent` (or numeric `0`/`1`/`2+`). Overridden by `-v`. |
| `prompts_dir`             | string    | (unset)        | Library directory for `-P/--prompt-file`. Tilde-expanded.                                          |
| `prompt_extensions`       | string[]  | see below      | Extensions probed when `-P <value>` doesn't include one. Default: `[".md", ".txt", ".prompt"]`.    |
| `file_content_template`   | template  | format default | Renders one file's content. See **Templates** below.                                               |
| `file_error_template`     | template  | format default | Renders when a file can't be read.                                                                 |
| `file_binary_template`    | template  | format default | Renders when a file is detected as binary.                                                         |
| `file_compact_template`   | template  | format default | Renders in compact mode (`-n 0`).                                                                  |
| `file_header_template`    | template  | format default | Shared partial usable from the four `file_*_templates` as `{{ template "file_header" . }}`.        |
| `section_template`        | template  | format default | Renders a logical section break (from `-s`).                                                       |
| `prompt_template`         | template  | format default | Renders text injected via `-p` or `-P`.                                                            |
| `tree_template`           | template  | format default | Renders the tree block from `-t`/`-T`.                                                             |
| `section_header_template` | template  | format default | Wraps the start of every section (XML uses this for `<content>`/`<section>`).                      |
| `section_footer_template` | template  | format default | Wraps the end of every section.                                                                    |
| `output_header_template`  | template  | format default | Rendered once before the first section.                                                            |
| `output_footer_template`  | template  | format default | Rendered once after the last section.                                                              |
| `stats_template`          | template  | format default | Renders the file/token summary footer.                                                             |

### Precedence for the prompts library

`-P/--prompt-file` resolves its library directory in this order (first match
wins):

1. `--prompts-dir <path>`
2. `$LX_PROMPTS_DIR`
3. `prompts_dir:` from the loaded config
4. `~/.config/lx/prompts`

See README's [Prompt library](README.md#prompt-library) section for resolver
semantics.

## Templates

`lx` uses Go's [`text/template`](https://pkg.go.dev/text/template). Each
template key receives a context value; the available fields and the helpers
in scope are listed below.

### Contexts

All field names come from `pkg/lx/core/types.go`.

**FileContext** — bound to `file_content_template`, `file_error_template`,
`file_binary_template`, `file_compact_template`, `file_header_template`:

| Field              | Type             | Notes                                              |
| ------------------ | ---------------- | -------------------------------------------------- |
| `Path`             | string           | Display path (relative when possible).             |
| `AbsPath`          | string           | Absolute path on disk.                             |
| `Size`             | int64            | Bytes. Pair with `humanize`.                       |
| `ModTime`          | time.Time        | Pair with `date "2006-01-02"`.                     |
| `TotalRows`        | int              | Lines after slicing.                               |
| `IsEstimate`       | bool             | True if rows are an estimate (e.g. binary skip).   |
| `Language`         | string           | Detected language hint for fenced code blocks.     |
| `Content`          | interface{}      | Pair with `endNewline`.                            |
| `IsBinary`         | bool             |                                                    |
| `IsImage`          | bool             |                                                    |
| `IsCompactView`    | bool             |                                                    |
| `IsError`          | bool             |                                                    |
| `ReadError`        | string           | Set when `IsError`.                                |
| `TokenEstimate`    | int64            |                                                    |
| `SkeletonMode`     | string           | `functions`, `types`, or empty.                    |
| `FileIndex`        | int              | 1-based index across the whole run.                |
| `SectionFileIndex` | int              | 1-based index within the current section.          |
| `Section`          | SectionContext   | Outer section info (notably `Section.TotalFiles`). |
| `Global`           | GlobalContext    | Whole-run info.                                    |

**SectionContext** — bound to `section_template`, `section_header_template`,
`section_footer_template`:

| Field        | Type          | Notes                                                                |
| ------------ | ------------- | -------------------------------------------------------------------- |
| `Body`       | string        | The section's title (from `-s "Title"`) or empty for implicit.       |
| `Index`      | int           | 1-based section index.                                               |
| `TotalFiles` | int           |                                                                      |
| `TotalSize`  | int64         |                                                                      |
| `IsImplicit` | bool          | True for the auto-section before the first `-s`.                     |
| `Global`     | GlobalContext |                                                                      |

**PromptContext** — bound to `prompt_template`:

| Field     | Type           |
| --------- | -------------- |
| `Body`    | string         |
| `Section` | SectionContext |
| `Global`  | GlobalContext  |

**TreeContext** — bound to `tree_template`: same shape as PromptContext, but
`Body` holds the rendered tree text (for PromptContext, `Body` holds the
injected prompt string).

**HeaderContext / FooterContext** — bound to `output_header_template` /
`output_footer_template`: a single field `Global` (GlobalContext).

**StatsContext** — bound to `stats_template`: `Global` only.

**GlobalContext** (reachable from every other context as `.Global`):
`TotalFiles`, `TotalSize`, `TotalWrittenBytes`, `TokenEstimate`,
`TotalSections`, `WorkDir`, `Metadata` (a `map[string]string`).

### Helper functions

Provided by `pkg/lx/templatex/defaults.go`:

| Helper       | Signature                       | Purpose                                                       |
| ------------ | ------------------------------- | ------------------------------------------------------------- |
| `humanize`   | `int64 → string`                | `1024` → `"1.0 kB"`, etc.                                     |
| `endNewline` | `interface{} → string`          | Stringify and ensure exactly one trailing `\n`.               |
| `date`       | `layout, time.Time → string`    | Wrapper around `time.Format`.                                 |
| `dataURI`    | `path → string`                 | Reads a file and emits a `data:<mime>;base64,…` URI.          |
| `escape`     | `interface{} → string`          | HTML-escape (used by the HTML format's defaults).             |

### Shared partials

The four `file_*_templates` and the `file_header_template` are parsed into a
single template set, so any of them can call

```gotemplate
{{ template "file_header" . }}
```

to render the shared header. The Markdown profile uses this; the XML profile
intentionally doesn't define `file_header_template` (see the comment at the
top of `xml_config.yaml`).

## Override recipes

### Just change one thing

```yaml
# ~/.config/lx/config.yaml
output_mode: copy
```

`lx <args>` now copies to clipboard automatically; templates and everything
else stay default.

### Switch to XML for one project

```bash
# .lx.yaml in the project root
echo "output_format: xml" > .lx.yaml
lx -y .lx.yaml src/
```

This loads the XML *format defaults* — you don't need to copy
`xml_config.yaml` unless you want to customize specific templates.

### Customize a single template

```yaml
# ~/.config/lx/config.yaml
file_binary_template: |-
  {{ template "file_header" . }} (binary, {{ .Size | humanize }} — skipped)
```

Other templates stay at format defaults.

### Per-shell switch

```bash
export LX_CONFIG=$HOME/dotfiles/lx-claude.yaml
```

Every `lx` invocation in that shell uses the file. Override per-command with
`-y`.
