# lx

[![Go Reference](https://pkg.go.dev/badge/github.com/rasros/lx.svg)](https://pkg.go.dev/github.com/rasros/lx)
[![Go Report Card](https://goreportcard.com/badge/github.com/rasros/lx)](https://goreportcard.com/report/github.com/rasros/lx)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

`lx` is a CLI tool that formats file system content for Large Language Models.

It replaces manual copy-pasting with a precise shell command, handling recursive discovery, formatting, and token
estimation automatically.

---

## Installation

Via go install:

```bash
go install github.com/rasros/lx/cmd/lx
```

Or via curl:

```bash
curl -fsSL https://raw.githubusercontent.com/rasros/lx/main/install.sh | bash
```

### System Dependencies (Clipboard)

The copy-to-clipboard feature (`-c` / `--copy`) depends on the following:

* **macOS**: Native support via `pbcopy`.
* **Windows**: Native support via `clip`.
* **Linux**: Requires `xclip` for X11 or `wl-clipboard` for Wayland to be installed.

If you're on Linux and not sure, check your session type:

```bash
echo $XDG_SESSION_TYPE
```

---

## Why lx?

* **Smart Discovery:** Respects `.gitignore` and `.lxignore` automatically (unlike `find`).
* **Binary Safety:** Detects and skips binary files to prevent corrupting your clipboard or token context.
* **Token Aware:** Estimates token usage (stderr) to keep you within LLM context limits.
* **Format Ready:** Wraps code in correct markdown fences with language tags for syntax highlighting.
* **Stream Processing:** Allows you to mix files, directories, prompts, and specific formatting rules in a single
  command.

---

## Basic Usage

Format a single file:

```bash
lx go.mod
```

**Output:**

````markdown
go.mod (11 rows)
---
```gomod
module github.com/rasros/lx

go 1.25.5

require gopkg.in/yaml.v3 v3.0.1

require (
        github.com/atotto/clipboard v0.1.4
        github.com/bmatcuk/doublestar/v4 v4.9.2
        github.com/monochromegane/go-gitignore v0.0.0-20200626010858-205db1a8cc00
)
```
````

Recursively walk a directory (respects `.gitignore`):

```bash
lx src/
```

Copy output directly to clipboard:

```bash
lx -c . # . is also the default if no arguments are provided
```

Copy all modified files in your current branch to the clipboard:

```bash
git diff --name-only | lx -c
```

---

## How It Works (The Paintbrush Model)

Unlike standard tools where flags apply globally, `lx` processes arguments left-to-right as a **stream**. Think of flags
as a "paintbrush": if you set a modifier, it applies to all **subsequent** files until it is changed or reset.

* **Apply limit to all:**
  `lx -n 50 file1.go file2.go` (Both files limited to 50 lines)

* **Apply limit to one, then reset:**
  `lx -n 50 file1.go -N file2.go` (`file1` is limited, `file2` is full length)

This allows you to construct precise context bundles: "Give me the full config file, but only the headers of the logs."

---

## Power Usage

Composing context from multiple sources with specific ordering and formatting:

```bash
lx -s "Server Code" -i "*.py" -e "test_*.py" src/server -E -s "Libs" requirements.txt -c
```

**Breakdown:**

1. **`-s "..."`**: Injects a Markdown section header ("Server Code").
2. **`-i` / `-e`**: Sets filters (include `*.py`, exclude `test_*.py`).
3. **`src/server`**: Walks the directory using those filters.
4. **`-E`**: Resets the active filters (so subsequent files aren't filtered).
5. **`-s`**: Injects a new section header ("Libs").
6. **`requirements.txt`**: Adds the dependency file.
7. **`-c`**: Copies the result to the clipboard.

---

## Formatting Modes

### Markdown (Default)

Standard fenced code blocks. Best for GitHub Copilot, ChatGPT, and DeepSeek.

### XML (Claude Optimized)

Formats output using XML tags (`<document>`, `<source>`, `<content>`). This is highly recommended for Anthropic's Claude
to ensure accurate parsing of multiple files.

```bash
lx --xml src/
```

### HTML (Shareable Reports)

Generates a standalone, syntax-highlighted HTML 5 file. Useful for attaching logs/code to bug tickets or sharing
snapshots with non-technical stakeholders. Includes responsive design (Pico CSS).

```bash
lx --html src/logs/ > report.html
```

---

## Piping & Integration

`lx` plays nicely with other tools. You can pipe a list of filenames from `fd`, `find`, or `ripgrep` directly into `lx`.

**Find files containing "TODO":**

```bash
rg -l "TODO" | lx -c
```

**Find all Rust files (using `fd`):**

```bash
fd -e rs | lx
```

---

## Configuration

`lx` is fully template-driven. You can override defaults by creating a config file at `~/.config/lx/config.yaml`.

**Sample Config:**

```yaml
# Default output behavior
output_mode: "copy"      # Options: stdout, copy
output_format: "xml"     # Options: markdown, xml, html
verbosity: "info"        # Options: debug, info, warn, error, silent

# Traversal rules
show_hidden: false
follow_symlinks: false
ignore: true             # Respect .gitignore

# Custom Templates (Go text/template syntax)
template: |
  ### {{ .Path }}
  {{ .Content }}

stats_template: |
  Total Files: {{ .Global.TotalFiles }}
  Tokens: {{ .Global.TokenEstimate }}
```

---

## Core Features

### 1. Slicing Files

Limit output to specific lines to save tokens.

```bash
# Get first 50 lines from the file
lx --head 50 error.log

# Compact mode: List filenames and sizes only (no content)
lx -n0 src/
```

### 2. Line Referencing

Add line numbers to help the LLM pinpoint specific locations.

```bash
lx -l server.log
```

### 3. Prompt Injection

Inject custom instructions directly into the stream without leaving the terminal.

```bash
lx -p "Refactor the following code to use Pydantic:" main.py
```

### 4. Stats Output

`lx` prints a summary of file counts, total size, and estimated tokens to stderr. This appears automatically when output
is redirected from stdout. You can disable it via `--no-stats`.