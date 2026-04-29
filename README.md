# lx

[![Go Reference](https://pkg.go.dev/badge/github.com/rasros/lx.svg)](https://pkg.go.dev/github.com/rasros/lx)
[![Go Report Card](https://goreportcard.com/badge/github.com/rasros/lx)](https://goreportcard.com/report/github.com/rasros/lx)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

lx is a CLI tool that bundles files into a single LLM-ready context.

It traverses directories, respects `.gitignore`, skips binaries by default, and outputs Markdown, XML, or HTML.
It can also fetch URL inputs, expand archives, extract text from office documents, and report estimated token usage.

lx reads arguments from left to right. You can change options mid-command, and each change applies to the files that
follow, so one command can mix different filters, slices, and formatting rules.

![demo](demo/demo.gif)

[Installation](#installation) • [Usage](#usage) • [Stream Processing](#stream-processing-model) • [Configuration](#configuration)

## Features

* Directory traversal that respects `.gitignore`, `.ignore`, and `.lxignore`.
* Output formats for Markdown (default), XML (Claude-friendly), and standalone HTML.
* URL input support without manual download.
* Archive expansion (`-Z`) for zip, tar, 7z, rar, and more.
* Document extraction (`-D`) for PDF, DOCX, XLSX, and PPTX.
* Structural views with tree output (`-t`/`-T`) and code skeleton extraction (`-u`/`-Y`).
* Direct clipboard copy (`-c`) and strong pipeline support (stdin, `-0`).

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

Clipboard support (`-c`) requires xclip on X11 Linux or wl-clipboard on Wayland. macOS and Windows work out of the
box.

## Usage

### Basic bundling
Grab everything in the current directory (ignoring hidden files) and copy to clipboard:
```bash
lx -c
```

### Tree overview only
Show project structure without file content:
```bash
lx -t src/
```

### Tree + content
Print a tree plus the same files as content:
```bash
lx -T src/
```

### Filter by type
Get all Python files, but skip the tests:
```bash
lx -i "*.py" -e "*test*" src/
```

### Function and type skeletons
Send only signatures and type definitions:
```bash
lx -u -Y src/
```

### Archive expansion
Expand a repository archive and include only Go files:
```bash
lx -Z -i "*.go" https://github.com/owner/repo/archive/refs/heads/main.zip
```

### Document extraction
Extract text from documents instead of treating them as binary:
```bash
lx -D docs/
```

### XML output
Dump the directory as structured XML (Claude prefers this):
```bash
lx --xml .
```

### HTML output
Generate a standalone, shareable HTML bundle:
```bash
lx --html src/ docs/
```

### URL input
Fetch a remote file and include it alongside local sources:
```bash
lx https://example.com/config.yaml src/
```

### Prompt injection
Prepend a custom instruction before the code context:
```bash
lx -p "Refactor the following code to use contexts:" main.go
```

### Prompt library
Reuse curated prompts from a directory (default `~/.config/lx/prompts`,
override with `--prompts-dir` or `$LX_PROMPTS_DIR`). See
[rasros/prompts](https://github.com/rasros/prompts) for an example library.
Lookup works by relative path, basename, or any path-like value:
```bash
export LX_PROMPTS_DIR=~/Workspaces/prompts
lx -P go/test src/foo.go -c          # nested by language/task
lx -P refactor src/                  # unambiguous basename
lx -P plan -P comments docs/ src/    # stack multiple prompts
lx --list-prompts                    # list everything in the library
```
Resolution order: literal path (if it looks like one) → `<libdir>/<value>` →
`<libdir>/<value>.{md,txt,prompt}` → recursive search by relative path or
basename. Ambiguous matches error with the candidate paths.

### Custom sections
Add custom headers to group specific files (also adjusts XML output):
```bash
lx -s "Code under test" src/database/users -s "Test fixtures" src/tests/fixtures
```

### Interactive selection with fzf
Use fd to find files, preview them with lx, and bundle the final selection:
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

Arguments are processed left to right as a stream of sections. Interleaved options (like `-n`, `--tail`, `-l`, `-i`)
are scoped to the section that follows.

Section boundaries are created when:

* An interleaved option appears after files.
* A section action (`-s`) is used.

At each boundary, interleaved options reset to defaults before new settings apply.

Example: Grab the last 50 lines of a log file, then src/ with line numbers enabled:
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
| `--bare`  | Bare     | Plain text concatenation with minimal output   |

## Configuration

You can set your defaults in `~/.config/lx/config.yaml`:
```yaml
output_mode: "stdout"    # stdout, copy
output_format: "xml"     # markdown, xml, html, bare
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
