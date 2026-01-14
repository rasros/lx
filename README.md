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
go install github.com/rasros/lx/cmd/lx@v1.1.0-rc.4
```

Or via curl:

```bash
curl -fsSL https://raw.githubusercontent.com/rasros/lx/main/install.sh | bash
```

---

## Basic Usage

Format a single file:

```bash
lx go.mod
```

This produces the following fenced output:

````markdown
go.mod (10 rows)
---
```gomod
module github.com/rasros/lx

go 1.25.5

require gopkg.in/yaml.v3 v3.0.1

require (
        github.com/atotto/clipboard v0.1.4
        github.com/monochromegane/go-gitignore v0.0.0-20200626010858-205db1a8cc00
)
```
````

Recursively walk a directory and output contents (respects `.gitignore`):

```bash
lx src/
```

Copy output directly to clipboard:

```bash
lx -c . # . is also the default if no arguments are provided
```

Write output to a file:

```bash
lx -o prompt.md src/
```

---

## Power Usage

`lx` allows you to compose context from multiple sources with specific ordering.

For example, to grab all server code (excluding tests), add the dependency file, organize them with headers, and
copy the result to your clipboard:

```bash
lx -s "Server Code" -i "*.py" -e "test_*.py" src/server -E -s "Libs" requirements.txt -c
```

**What just happened?**

1. **`-s "..."`**: Injects a Markdown section header.
2. **`-i` / `-e`**: Sets up filters to only include Python files and exclude tests.
3. **`src/server`**: Recursively walks the source directory using those filters.
4. **`-E`**: Resets the active filters so subsequent files aren't filtered.
5. **`-s`**: Injects a new section header.
6. **`requirements.txt`**: Appends the specific dependency file.
7. **`-c`**: Copies the entire formatted output to your clipboard.

---

## Piping & Integration

`lx` plays nicely with other tools. You can pipe a list of filenames from `fd`, `find`, or `ripgrep` directly into `lx`.

**Using `ripgrep` to find files containing "TODO":**

```bash
rg -l "TODO" | lx -c
```

**Using `fd` to find all Rust files:**

```bash
fd -e rs | lx
```

---

## XML Support (Recommended for Claude)

You can switch to XML formatting, which is recommended by Anthropic's documentation for Claude to ensure better parsing
of long contexts.

```bash
lx --xml src/
```

---

## HTML Support (Recommended for Browser Viewing)

`lx` can output a complete, minimalistic HTML document styled with Pico CSS. This is useful for creating shareable
archives or viewing code in a browser. It automatically detects images and renders them too.

```bash
lx --html src/ > output.html
```

---

## Core Features

### 1. Smart Discovery

`lx` works like `ripgrep` or `fd`. It recursively walks directories while automatically respecting `.gitignore`,
`.ignore`, and `.lxignore` files.

```bash
lx src/  # Walks src/, skipping ignored files
lx -H .  # Includes hidden files
```

### 2. Slicing Files

Limit output to specific lines to save tokens.

```bash
# Get 50 lines from the middle of a file
lx --lines 50 error.log

# Compact mode: List filenames and sizes only (no content)
lx -n0 src/
```

### 3. Line Referencing

Add line numbers to help the LLM pinpoint specific locations in code or logs.

```bash
lx -l server.log
```

### 4. Prompt Injection

Inject custom instructions directly into the stream without leaving the terminal with `-s` or `-p`.

```bash
lx -p "Refactor the following code to use Pydantic:" main.py
```

**Tip:** You can create aliases for common prompts. Add this to your shell profile to quickly inject your test
prompt:

```bash
alias lxt='lx -p "$(cat ~/prompts/tests.md)"'
```

### 5. Stats Output

`lx` prints a summary of file counts, total size, and estimated tokens to stderr. This appears automatically when
output is redirected from stdout. You can disable it by `--no-stats`.

### 6. File Support

Binary files are printed as a single line and do not cause errors.

PDF conversion and compressed archives are planned as future improvements.

---

## Configuration

`lx` is fully template-driven. You can customize the Markdown output format by creating a config file at
`~/.config/lx/config.yaml`.
