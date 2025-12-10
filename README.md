# lx

[![Go Reference](https://pkg.go.dev/badge/github.com/rasros/lx.svg)](https://pkg.go.dev/github.com/rasros/lx)
[![Go Report Card](https://goreportcard.com/badge/github.com/rasros/lx)](https://goreportcard.com/report/github.com/rasros/lx)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**Converts files into simple Markdown-fenced blocks for easy use in LLM chats.**

The goal is to make prompt setup **repeatable** and **precise**. Instead of letting an agent guess context or manually selecting files in a UI, you define the exact context you want in one shell command and rerun it whenever you need a fresh session. It works smoothly with modern tools like `rg -l`, `fd` (or `grep` and `find`), and recursive shell globs.

This gives you a stable, controllable workflow:

- You decide exactly what context the model sees.
- If a conversation drifts, you restart instantly with the same context.
- Adjust the command, rerun it, and paste the updated output.

---

## Installation

Via go install:
```bash
go install github.com/rasros/lx/cmd/lx@latest
````

Or via curl into `$HOME/.local/bin/lx`:
```
curl -fsSL https://raw.githubusercontent.com/rasros/lx/main/install.sh | bash
```

---

## Basic usage

Format a single file as an LLM-ready snippet:

```bash
lx cmd/lx/main.go
```

Example output (trimmed):

~~~text
cmd/lx/main.go (18 rows)
---
```go
package main

import (
    "fmt"
... (rest omitted)
```
~~~

You can put it directly in your clipboard by piping it to a copy tool:
```bash
# Wayland (Ubuntu, Debian)
lx file.py | wl-copy
```

```bash
# X11
lx file.py | xclip -selection clipboard
# or
lx file.py | xsel --clipboard --input
```

```bash
# macOS
lx file.py | pbcopy
```

```bash
# MSYS2
lx file.py | clip
```

---

## Features

* Generates Markdown headers for one or many files.
* Automatically detects language from file extension.
* Supports simple licing (`-h`, `-t`, `-n`).
* Optional line numbers when needed.
* Reads filenames from CLI args or stdin.
* Customizable delimiters with placeholders.

---

## More examples

### Filtering file names
We can select multiple files in shells that allow recursive glob:
```bash
lx **/*.py
```

If you need to exclude certain files we rely on standard tools for file selection.

This example uses includes all python files except those with name ending in `_test.py`. Here `fd` or `find`:
```bash
# find using stdin-mode
find . -name '*.py' ! -name '*_test.py' | lx

# fd using stdin-mode
fd -e py -E "*_test.py" | lx
```

Or through shell glob syntax:
```bash
# zsh glob
lx **/*.py~*_test.py 

# bash glob
shopt -s globstar extglob
lx **/!(*_test).py

# fish glob
lx **/*.py ^**/*_test.py

# MSYS2 glob
shopt -s globstar extglob
lx **/!(*_test).py
```

### Pattern search
Searching for patterns is easily done through `grep -l` and pipe matching files to `lx`.

This example searches for files with a function starting with `save` under the src folder:
```bash
# grep
grep -rl "def save" src | lx

# ripgrep
rg -l "def save" src | lx
```

### Line numbers: `-l`

TOON supports line numbers so we do too 🤷.

This is actually very useful for letting the LLM reference where in a big log dump something is wrong. Or if you specifically include prompting about instructions about line number references.

```bash
lx -l server.log
```

### Slicing: `-t -h -n`

While iterating on the command it's convenient to slice files so you can more easily see what's included:

```bash
# First 40 lines
lx -h40 server.log

# Last 80 lines
lx -t80 server.log

# Both ends (split 60 lines between head & tail)
lx -n60 server.log
```

Short forms like `-h5`, `-t10`, `-n2` are supported.

### Custom delimiters and placeholders

Default delimiters (excluding new-lines):

~~~text
{filename} ({row_count} rows)
---
```{language}
...file contents...
```
~~~

Override them:

~~~bash
lx \
  --prefix-delimiter="### {filename}{n}```{language}{n}" \
  --postfix-delimiter="```{n}{n}" \
  file.go
~~~

Placeholders:

* `{n}` – OS specific newline character(s)
* `{filename}` – relative path of the file from current directory
* `{row_count}`
* `{byte_size}`
* `{last_modified}`
* `{language}` – derived from file ending used for markdown syntax highlighting

