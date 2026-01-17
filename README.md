# lx

[![Go Reference](https://pkg.go.dev/badge/github.com/rasros/lx.svg)](https://pkg.go.dev/github.com/rasros/lx)
[![Go Report Card](https://goreportcard.com/badge/github.com/rasros/lx)](https://goreportcard.com/report/github.com/rasros/lx)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

`lx` is a CLI context bundler for Large Language Models. It recursively discovers files, respects ignore rules, detects
binaries, and formats content into Markdown, XML, or HTML with built-in token estimation.

## Installation

Via go install:

```bash
go install github.com/rasros/lx/cmd/lx@latest
```

Or via curl:

```bash
curl -fsSL https://raw.githubusercontent.com/rasros/lx/main/install.sh | bash
```

### Dependencies

Clipboard support (`-c`) requires system utilities:

* **Linux (X11):** `xclip`
* **Linux (Wayland):** `wl-clipboard`
* **macOS/Windows:** Native support (`pbcopy` / `clip`) included.

---

## Stream Processing Model

Unlike standard CLI tools where flags are global, `lx` treats command-line arguments as a sequential stream.

* **Global Options:** Apply to the entire execution (e.g., `-c`).
* **State Modifiers:** Apply to all **subsequent** files until reset or changed.
* **Actions:** The inputs to process (files, directories, or generators like `-p`).

This allows for granular control within a single command:

```bash
# Usage: lx [OPTIONS] [state modifiers] <actions>...
lx -n 50 log.txt -N main.go
```

1. `-n 50` (State Modifier): Sets line limit to 50 for subsequent files.
2. `log.txt` (Action): Processed with line limit.
3. `-N` (State Modifier): Resets line limits.
4. `main.go` (Action): Processed in full.

---

## Usage Examples

### Basic Formatting

Recursively walk the current directory (default action), respecting `.gitignore`, and print to stdout.

```bash
lx
```

### Clipboard Integration

Bundle the `src` directory and copy directly to the system clipboard.

```bash
lx -c src/
```

### Git Integration

Pipe a list of modified files from git directly into `lx`.

```bash
git diff --name-only | lx -c
```

### Filtering and Slicing

Include only Python files, exclude tests, and inject a header.

```bash
lx -s "Backend Logic" -i "*.py" -e "test_*" src/
```

### Token Economy (Head/Tail)

Read the last 100 lines of a log file and the full content of a config file.

```bash
lx --tail 100 app.log -N config.yaml
```

### Prompt Injection

Inject instructions directly into the context stream.

```bash
lx -p "Analyze the following error logs:" error.log
```

---

## Output Formats

### Markdown (Default)

Standard fenced code blocks. Optimized for GitHub Copilot, ChatGPT, and DeepSeek.

### XML (`--xml`)

Wraps content in `<document>`, `<source>`, and `<content>` tags. This is recommended by Anthropic for Claude.

```bash
lx --xml src/
```

### HTML (`--html`)

Generates a standalone HTML file with syntax highlighting (Pico CSS). Useful for viewing in a browser.

```bash
lx --html src/logs/ > debug_report.html
```

---

## Configuration

Defaults can be overridden via `~/.config/lx/config.yaml`.

```yaml
# Output settings
output_mode: "stdout"    # stdout, copy
output_format: "markdown" # markdown, xml, html
verbosity: "info"

# Traversal
show_hidden: false
follow_symlinks: false
ignore: true             # Respect .gitignore/.lxignore

# Templates (Go text/template)
# Available variables: .Path, .Content, .Extension
file_content_template: |
  ### {{ .Path }}
  {{ .Content }}

stats_template: |
  ---
  Files: {{ .Global.TotalFiles }}
  Tokens: {{ .Global.TokenEstimate }}
```

---

## Flag Reference

### Global Options

*Settings that apply to the whole operation, regardless of position.*

* `-c`, `--copy`: Copy the final formatted output to the system clipboard.
* `-o`, `--output <value>`: Write the result to a file path.
* `--md`: Format output as Markdown (default).
* `--xml`: Format output using XML tags (Claude optimized).
* `--html`: Format output as a standalone HTML 5 file.
* `-C`, `--stdout`: Force output to stdout.
* `-q`, `--quiet`: Suppress stats summary and logging.
* `-v`, `--verbose`: Set log level.

### Discovery Options

*Settings that control how files are found when walking directories.*

* `-H`, `--hidden`: Include hidden files and directories.
* `-S`, `--follow`: Follow symbolic links to directories.
* `--links` / `--no-links`: Include/ignore symbolic links to files.
* `--ignore` / `-I`, `--no-ignore`: Toggle respect for `.gitignore` and ignore files.
* `-0`, `--null`: Expect NUL-terminated filenames from stdin.

### State Modifiers

*Flags that alter the processing rules for subsequent files.*

* `-n`, `--lines <n>`: Limit subsequent files to N lines (0 for compact).
* `--head <n>`: Read only the first N lines of subsequent files.
* `--tail <n>`: Read only the last N lines of subsequent files.
* `-N`, `--reset-lines`: Reset slicing rules; print full content.
* `-l`, `--line-numbers`: Enable line numbers.
* `-L`, `--reset-line-numbers`: Disable line numbers.
* `-i`, `--include <value>`: Add a glob whitelist pattern.
* `-e`, `--exclude <value>`: Add a glob blacklist pattern.
* `-E`, `--reset-filters`: Clear all active include/exclude filters.

### Actions

*Explicit content generators that insert data into the stream immediately.*

* `[path]`: File or directory to process.
* `-f`, `--file <value>`: Force process a specific path (bypasses ignores/filters).
* `-s`, `--section <value>`: Insert a logical separator with a header.
* `-p`, `--prompt <value>`: Inject a custom text prompt.