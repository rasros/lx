# lx

[![Go Reference](https://pkg.go.dev/badge/github.com/rasros/lx.svg)](https://pkg.go.dev/github.com/rasros/lx)
[![Go Report Card](https://goreportcard.com/badge/github.com/rasros/lx)](https://goreportcard.com/report/github.com/rasros/lx)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

`lx` is a CLI tool that bundles your codebase into a single context string for LLMs.

It crawls directories, respects your gitignores, skips binaries, and outputs Markdown, XML, or HTML directly to 
clipboard along with an estimated token count.

Instead of blindly concatenating files together, `lx` processes arguments as a stream. This means you can apply
different rules (like line limits or filters) to specific files on the fly within a single command.

---

[Installation](#installation) • [Usage](#usage) • [Stream Processing](#stream-processing-model) • [Configuration](#configuration)

## Features

* Token estimation for individual files and the final output.
* Smart traversal that respects `.gitignore`, `.ignore`, and `.lxignore`.
* Stream processing to apply interleaved options (like `--tail 50`) mid-command.
* Outputs standard Markdown, XML (ideal for Claude), or HTML.
* Copies directly to your system clipboard on Linux, macOS, and Windows.
* Plays nice with Unix pipelines (`find`, `fd`, `git`, etc.).

## Installation

### Go install
```bash
go install github.com/rasros/lx/cmd/lx@latest
```

### Curl script (pre-built binaries)
```bash
curl -fsSL https://raw.github.com/rasros/lx/main/install.sh | bash
```

### Dependencies

Clipboard support (`-c`) requires `xclip` on X11 Linux or `wl-clipboard` on Wayland. macOS and Windows work out of the
box.

## Usage

### Basic bundling
Grab everything in the current directory (ignoring hidden files) and copy to clipboard:
```bash
lx -c
```

### Filter by type
Get all Python files, but skip the tests:
```bash
lx -i "*.py" -e "*test*" src/
```

### XML output
Dump the directory as structured XML (Claude prefers this):
```bash
lx --xml .
```

### Prompt injection
Prepend a custom instruction before the code context:
```bash
lx -p "Refactor the following code to use contexts:" main.go
```

### Custom sections
Add custom headers to group specific files (also adjusts XML output):
```bash
lx -s "Code under test" src/database/users -s "Test fixtures" src/tests/fixtures
```

### Interactive selection with fzf
Use `fd` to find files, preview them with `lx`, and bundle the final selection:
```bash
fd -t f | fzf -m --preview 'lx -n 20 {}' | lx -c
```

### Piping to LLM CLI
Send context directly into [llm](https://github.com/simonw/llm) tool:
```bash
lx -p "Explain this project structure" src/ | llm
```

### Working with git
Only bundle the files that changed in your current branch:
```bash
git diff --name-only main | lx -c
```

## Stream Processing Model

`lx` processes arguments left to right as a stream of **sections** - consecutive file/directory arguments with
no interleaved options between them. Interleaved options (like `-n`, `--tail`, `-l`, `-i`) are scoped to the section
they precede. When an interleaved option appears *after* files, it creates a **section boundary**: all interleaved
options reset to defaults before the new option takes effect.

Example: Grab the last 50 lines of a log file, then `src/` with line numbers enabled:
```bash
lx --tail 50 app.log -l src/
```

| Argument    | Effect                                                                  |
|-------------|-------------------------------------------------------------------------|
| `--tail 50` | Sets tail for the next section                                          |
| `app.log`   | Gets the last 50 lines                                                  |
| `-l`        | Section boundary: state resets to defaults, then line numbers enabled   |
| `src/`      | Gets full content with line numbers                                     |

## Output Formats

| Flag      | Format   | Best For                                       |
|-----------|----------|------------------------------------------------|
| (default) | Markdown | ChatGPT, GitHub Copilot, DeepSeek              |
| `--xml`   | XML      | Claude (uses `<document>` and `<source>` tags) |
| `--html`  | HTML     | Archiving or visual debugging                  |

## Configuration

You can set your defaults in `~/.config/lx/config.yaml`:
```yaml
output_mode: "stdout"    # stdout, copy
output_format: "xml"     # markdown, xml, html
```

## Comparison

| Feature           | lx | repopack | files-to-prompt |
|-------------------|----|----------|-----------------|
| Language          | Go | Node.js  | Python          |
| Stream Processing | ✅  | ❌        | ❌               |
| Clipboard Copy    | ✅  | ❌        | ❌               |
| XML Support       | ✅  | ✅        | ✅               |
| Token Estimation  | ✅  | ✅        | ❌               |
| Binary Detection  | ✅  | ✅        | ✅               |
