# lx

[![Go Reference](https://pkg.go.dev/badge/github.com/rasros/lx.svg)](https://pkg.go.dev/github.com/rasros/lx)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

lx turns a pile of files into one prompt you can paste into an LLM.

Point it at a directory and it walks the tree, honours your ignore files, leaves binaries alone, and prints the result as Markdown, XML, HTML, or plain text. It adds a token estimate so you know what you're about to spend.

![demo](demo/demo.gif)

[Install](#install) · [Everyday use](#everyday-use) · [Where files come from](#where-files-come-from) · [Stream processing](#stream-processing) · [Configuration](#configuration)

## Install

```bash
go install github.com/rasros/lx/cmd/lx@latest
```

Or grab a pre-built binary:

```bash
curl -fsSL https://raw.github.com/rasros/lx/main/install.sh | bash
```

The clipboard flag (`-c`) needs xclip (X11) or wl-clipboard (Wayland) on Linux; macOS and Windows already have what they
need.

## Everyday use

The common case is "bundle this project and put it on my clipboard":

```bash
lx -c
```

With no arguments `lx` starts from the current directory, but you can name any paths you like:

```bash
lx src/ docs/
```

It respects `.gitignore`, `.ignore`, and `.lxignore`, skips hidden files, and drops binaries, so you usually get
sensible output without asking for it.

**Get your bearings first.** `-t` prints just the directory tree; `-T` prints the tree *and* the files under it:

```bash
lx -t src/
```

**Skip the bodies.** `-u` and `-Y` extract function signatures and type definitions instead of full source, which is
often all a model needs to reason about a codebase:

```bash
lx -u -Y src/
```

**Narrow things down** with include/exclude globs. Here, Python without the tests:

```bash
lx -i "*.py" -e "*test*" src/
```

Since `lx` reads paths from stdin, it composes with whatever you already use to pick files:

```bash
git diff --name-only main | lx -c                  # everything you changed
fd -t f | fzf -m --preview 'lx -n 20 {}' | lx -c   # pick interactively
```

**Attach a prompt** and pipe it straight into a tool like [llm](https://github.com/simonw/llm):

```bash
lx -p "Explain this project structure" src/ | llm
lx -p "Refactor this to use contexts:" main.go
```

Without `-c` or `-o`, output goes to stdout.

## Where files come from

Local paths are the default, but they aren't the only option.

A URL can sit anywhere a path can:

```bash
lx https://example.com/config.yaml src/
```

Short repository URLs are pulled down as archives, so you can bundle a project you haven't cloned. GitHub, GitLab,
Bitbucket, and Codeberg all work:

```bash
lx github.com/owner/repo
lx https://gitlab.com/owner/repo/-/tree/dev
```

Local archives expand with `-Z` (zip, tar, 7z, rar, and friends):

```bash
lx -Z archive.zip
```

And `-D` pulls readable text out of PDFs, Word docs, spreadsheets, and slide decks that would otherwise be skipped as
binary:

```bash
lx -D docs/
```

Media files stay binary, but `-M` describes them from their container header, so the model at least knows what is there:

```bash
lx -M assets/
```

```
Container: mp4
Duration: 00:04:12.480
Bitrate: 2.4 Mbps
Video: h264, 1920x1080, 25.00 fps
Audio: aac, 48000 Hz, stereo
```

## Output formats

Markdown is the default. The rest are one flag each:

```bash
lx --xml .            # XML, handy for models that like tags
lx --html src/ docs/  # a standalone HTML page
lx --bare file.txt    # plain text, almost no wrapping
```

## Prompts and sections

Reusable prompts live in `~/.config/lx/prompts` (override with `$LX_PROMPTS_DIR` or `--prompts-dir`). Reference them by
path, filename, or any unambiguous basename. An ambiguous name just prints the candidates so you can be more specific.

```bash
lx -P refactor src/
lx -P go/test src/foo.go -c
lx -P plan -P comments docs/ src/   # stack as many as you want
lx --list-prompts
```

[rasros/prompts](https://github.com/rasros/prompts) is an example library to start from.

Use `-s` to label groups of files, which also become section names in XML output:

```bash
lx -s "Code under test" src/database/users \
   -s "Test fixtures"   src/tests/fixtures
```

## Stream processing

This is the one idea worth understanding: `lx` reads arguments left to right, and an option applies to the files that
come *after* it.

```bash
lx --tail 50 app.log -l src/
```

That takes the last 50 lines of `app.log`, then `src/` with line numbers: two different treatments in one command. When
an option (or `-s`) appears after a file, `lx` opens a fresh section and resets the per-section options, so settings
don't leak from one group into the next:

```bash
lx --tail 50 app.log \
   -u src/ \
   -i "*.md" docs/
```

The tail of a log, code skeletons from `src/`, and the Markdown under `docs/`, each on its own terms.

## Configuration

Drop a `~/.config/lx/config.yaml` to change the defaults; anything you leave out keeps its built-in value.

```yaml
output_mode: "copy"    # stdout | copy
output_format: "xml"   # markdown | xml | html | bare
prompts_dir: "~/Workspaces/prompts"
```

Two ready-made profiles ship with the repo: [`default_config.yaml`](default_config.yaml) and 
[`xml_config.yaml`](xml_config.yaml). Point at one for a single run with `-y`, or set `$LX_CONFIG` to make it the
default:

```bash
lx -y xml_config.yaml src/
```

[CONFIG.md](CONFIG.md) has the full reference, including template context and helpers.

## Coding-agent skill

[`skills/lx/SKILL.md`](skills/lx/SKILL.md) teaches coding agents to reach for `lx` when they explore a codebase. Symlink
it wherever your agent looks for skills:

```bash
ln -s "$(pwd)/skills/lx" ~/.claude/skills/lx
ln -s "$(pwd)/skills/lx" ~/.config/opencode/skills/lx
```
