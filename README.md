# lx

[![Go Reference](https://pkg.go.dev/badge/github.com/rasros/lx.svg)](https://pkg.go.dev/github.com/rasros/lx)
[![Go Report Card](https://goreportcard.com/badge/github.com/rasros/lx)](https://goreportcard.com/report/github.com/rasros/lx)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

`lx` is a CLI context bundler for Large Language Models.

It recursively discovers files, respects ignore rules, detects binaries, and formats content into Markdown, XML, or HTML
with built-in token estimation.

Unlike simple file concatenators, `lx` treats arguments as a stream. This allows you to apply state modifiers (like line
limits or filters) to specific subsets of files within a single command.

---

[Installation](#installation) • [Usage](#usage) • [Stream Processing](#stream-processing-model) • [Configuration](#configuration)

## Features

* **Token Economy:** Automatically calculates token estimates for every file and the total bundle.
* **Smart Traversal:** Respects `.gitignore`, `.ignore`, and `.lxignore`. Skips binary files by default.
* **Stream Processing:** Apply different rules (e.g., `--tail 50`) to different files in one pass.
* **LLM Optimized:** Outputs Markdown (GitHub/OpenAI), XML (Claude), or HTML (visual debugging).
* **Clipboard Ready:** Pipes output directly to the system clipboard on Linux (`xclip`/`wl-copy`), macOS, and Windows.
* **Pipeline Friendly:** Accepts file lists via stdin from tools like `find`, `fd`, or `git`.

## Installation

### Via Go install

```bash
go install github.com/rasros/lx/cmd/lx@latest
```

### Via Curl (Pre-built binaries)

```bash
curl -fsSL https://raw.github.com/rasros/lx/main/install.sh | bash
```

### Dependencies

Clipboard support (`-c`) requires:

* **Linux (X11):** `xclip`
* **Linux (Wayland):** `wl-clipboard`
* **macOS / Windows:** Native support included.

## Usage

### Basic Bundling

Walk the current directory, ignoring hidden/gitignored files, and copy to clipboard:

```bash
lx -c
```

### Git Integration

Bundle only the files changed in the current branch:

```bash
git diff --name-only main | lx -c
```

### Filter by Type

Find Python files, exclude tests, and output as XML (Recommended for **Anthropic/Claude**):

```bash
lx --xml -i "*.py" -e "*test*" src/
```

### Prompt Injection

Inject a custom instruction header before the file context:

```bash
lx -p "Refactor the following code to use contexts:" main.go
```

## Stream Processing Model

Arguments are processed sequentially. Flags are not global; they are **state modifiers** that apply to all subsequent
actions until reset.

1. **Modifiers:** (`-n`, `--tail`, `-i`) Apply to subsequent files.
2. **Actions:** (`file`, `dir/`) Capture the current state.
3. **Resets:** (`-N`, `-E`) Clear active modifiers.

**Example:**
Bundling a log file (last 50 lines) and the full source code in one command.

```bash
lx --tail 50 app.log -N src/main.go
```

| Argument      | Type         | Effect                                 |
|:--------------|:-------------|:---------------------------------------|
| `--tail 50`   | **Modifier** | Sets "Read Strategy" to last 50 lines. |
| `app.log`     | **Action**   | Processed using the tail strategy.     |
| `-N`          | **Reset**    | Resets strategy to full content.       |
| `src/main.go` | **Action**   | Processed in full.                     |

## Integration

### Using with `fzf`

Use `fd` to find files, `lx` to preview them, and `lx` again to bundle the selection.

```bash
# Requires: fd, fzf
fd -t f | fzf -m --preview 'lx -n 20 {}' | lx -c
```

### Using with `llm`

Pipe context directly into [llm](https://github.com/simonw/llm) tool:

```bash
lx -p "Explain this project structure" src/ | llm
```

## Output Formats

| Flag      | Format       | Best For                                                                 |
|:----------|:-------------|:-------------------------------------------------------------------------|
| (default) | **Markdown** | ChatGPT, GitHub Copilot, DeepSeek. Uses fenced code blocks.              |
| `--xml`   | **XML**      | Claude (Anthropic). Uses semantic tags `<document>`, `<source>`.         |
| `--html`  | **HTML**     | Archiving/Sharing. Generates a standalone file with syntax highlighting. |

## Configuration

Defaults can be overridden via `~/.config/lx/config.yaml`.

```yaml
output_mode: "stdout"    # stdout, copy
output_format: "xml"     # markdown, xml, html
show_hidden: false
follow_symlinks: false
ignore: true             # Respect .gitignore
```

## Comparison

| Feature               | lx | repopack | files-to-prompt |
|:----------------------|:--:|:--------:|:---------------:|
| **Language**          | Go | Node.js  |     Python      |
| **Stream Processing** | ✅  |    ❌     |        ❌        |
| **Clipboard Copy**    | ✅  |    ❌     |        ❌        |
| **XML Support**       | ✅  |    ✅     |        ✅        |
| **Token Estimation**  | ✅  |    ✅     |        ❌        |
| **Binary Detection**  | ✅  |    ✅     |        ✅        |
