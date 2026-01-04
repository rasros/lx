# lx

[![Go Reference](https://pkg.go.dev/badge/github.com/rasros/lx.svg)](https://pkg.go.dev/github.com/rasros/lx)
[![Go Report Card](https://goreportcard.com/badge/github.com/rasros/lx)](https://goreportcard.com/report/github.com/rasros/lx)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

`lx` makes prompt setup **repeatable** and **precise**. Instead of letting an agent guess context or manually selecting files in a UI, define the exact context in one shell command and rerun it whenever you need a fresh session. It works smoothly with standard tools like `rg -l`, `fd`, `grep`, `find`, and shell globs.

---

## Installation

Via go install:
```bash
go install [github.com/rasros/lx/cmd/lx@latest](https://github.com/rasros/lx/cmd/lx@latest)
```

Or via curl:
```bash
curl -fsSL [https://raw.githubusercontent.com/rasros/lx/main/install.sh](https://raw.githubusercontent.com/rasros/lx/main/install.sh) | bash
```

---

## Basic usage

Format a single file as an LLM-ready snippet:

```bash
lx cmd/lx/main.go
```

Copy output directly to clipboard:

```bash
lx -c cmd/lx/main.go
```

Write output to a file:

```bash
lx -o prompt.md cmd/lx/main.go
```

---

## Features

* **Smart Formatting:** Generates Markdown headers with row counts and language detection.
* **Context Control:** Add custom prompts (`-p`) and section headers (`-s`) directly in the stream.
* **Slicing:** Use `-n` to limit output lines or `-n0` for compact views.
* **Line Numbers:** Optional `-l` flag for referencing specific lines.
* **Input Flexibility:** Reads filenames from arguments or stdin pipe.
* **Configurable:** Fully template-based output via `config.yaml`.

---

## Examples

### Filtering files
Select multiple files using globs or pipe from other tools:

```bash
# Shell glob
lx **/*.py

# Using fd (exclude tests)
fd -e py -E "*_test.py" | lx

# Using grep (files containing "TODO")
grep -rl "TODO" src | lx
```

### Adding Context
Inject instructions and headers into the prompt stream:

```bash
lx -p "Refactor the following code to use interfaces:" \
   -s "Current Implementation" \
   src/old_impl.go
```

### Slicing & Compact Mode
Useful for large logs or getting a quick overview:

```bash
# Print N lines (split between head and tail)
lx -n 20 server.log

# Compact mode (print filename and stats only, no content)
lx -n 0 server.log
```

Binary files are written in compact mode.

### Line Numbers
Helpful for asking the LLM to point out specific errors:

```bash
lx -l main.go
```

---

## Configuration

`lx` uses Go templates for rendering. You can customize the output format by creating a config file at `~/.config/lx/config.yaml` (Linux/Mac) or `%APPDATA%\lx\config.yaml` (Windows).

See `default_config.yaml` in the repo for all available options and template variables.