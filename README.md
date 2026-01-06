# lx

[![Go Reference](https://pkg.go.dev/badge/github.com/rasros/lx.svg)](https://pkg.go.dev/github.com/rasros/lx)
[![Go Report Card](https://goreportcard.com/badge/github.com/rasros/lx)](https://goreportcard.com/report/github.com/rasros/lx)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

`lx` makes prompt setup **repeatable** and **precise**. Instead of letting an agent guess context or manually selecting
files in a UI, define the exact context in one shell command and rerun it whenever you need a fresh session. It works
smoothly with standard tools like `grep` and shell globs.

---

## Installation

Via go install:

```bash
go install github.com/rasros/lx/cmd/lx@v1.1.0-rc.2
```

Or via curl:

```bash
curl -fsSL https://raw.githubusercontent.com/rasros/lx/main/install.sh | bash
```

---

## Basic usage

Format a single file:

```bash
lx cmd/lx/main.go
```

Recursively walk a directory (respects `.gitignore` by default):

```bash
lx src/
```

Copy output directly to clipboard:

```bash
lx -c .
```

Write output to a file:

```bash
lx -o prompt.md src/
```

---

## Features

* **Smart Formatting:** Generates Markdown headers with row counts and language detection.
* **Recursive Discovery:** Walks directories similar to `fd` or `ripgrep`.
* **Ignore Logic:** Respects `.gitignore`, `.ignore`, and `.lxignore` automatically.
* **Context Control:** Add custom prompts (`-p`) and section headers (`-s`) directly in the stream.
* **Slicing:** Use `-n` to limit output lines or `-n0` for compact views.
* **Line Numbers:** Optional `-l` flag for referencing specific lines.
* **Input Flexibility:** Reads filenames from arguments or stdin pipe.
* **Configurable:** Fully template-based output via `config.yaml`.

---

## Examples

### Discovery & Filtering

```bash
# Walk current directory, respecting gitignore
lx .

# Include hidden files
lx -H .

# Include ignored files (disable gitignore logic)
lx -I .

# Follow symbolic links
lx -L .
```

### Adding Context

Inject instructions and headers into the prompt stream:

```bash
lx -p "Implement pytests for the following files. Follow the same structure as in the existing tests." \
   -s "Files to test" \
   src/foo/*.py \
   -s "Test fixtures and sample tests." \
   src/tests/fixtures src/tests/bar_test.py
```

### Slicing & Compact Mode

Useful for large logs or getting a quick overview:

```bash
# Print N lines (split between head and tail), --head and --tail are also supported
lx -n100 server.log

# Compact mode (print filename and stats only, no content)
lx -n0
```

Binary files are written in compact mode automatically.

### Line Numbers

Helpful for asking the LLM to point out log file issues:

```bash
lx -l server.log
```

---

## Configuration

`lx` uses Go templates for rendering. You can customize the output format by creating a config file at
`~/.config/lx/config.yaml` (Linux/Mac) or `%APPDATA%\lx\config.yaml` (Windows).

See `default_config.yaml` in the repo for all available options and template variables.
