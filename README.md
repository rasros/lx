# lx

[![Go Reference](https://pkg.go.dev/badge/github.com/rasros/lx.svg)](https://pkg.go.dev/github.com/rasros/lx)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

lx turns a pile of files into one prompt you can paste into an LLM.

Point it at a directory and it walks the tree, honours your ignore files, leaves binaries alone, and prints the result as
Markdown, XML, HTML, or plain text, with a token estimate so you know what you're about to spend.

![demo](demo/demo.gif)

## Install

```bash
go install github.com/rasros/lx/cmd/lx@latest       # or:
curl -fsSL https://raw.github.com/rasros/lx/main/install.sh | bash
```

The clipboard flag (`-c`) needs xclip (X11) or wl-clipboard (Wayland) on Linux; macOS and Windows already have what they
need.

## Everyday use

With no arguments lx bundles the current directory to stdout, respecting .gitignore, .ignore, and .lxignore,
skipping hidden files, and dropping binaries. Name any paths you like, and add `-c` to copy instead:

```bash
lx -c                          # this project, on the clipboard
lx src/ docs/
```

```bash
lx -t src/                     # just the tree (-T for tree plus files)
lx -u -Y src/                  # signatures and type definitions, no bodies
lx -i "*.py" -e "*test*" src/  # include/exclude globs
lx -n 20 src/                  # first 20 lines of each file
```

Paths can come from stdin, so lx composes with whatever you already use to pick files:

```bash
git diff --name-only main | lx -c                  # everything you changed
fd -t f | fzf -m --preview 'lx -n 20 {}' | lx -c   # pick interactively
```

Attach a prompt with `-p` and pipe it into a tool like [llm](https://github.com/simonw/llm):

```bash
lx -p "Explain this project structure" src/ | llm
lx -P go/test src/foo.go | llm          # a saved prompt from ~/.config/lx/prompts
```

Markdown is the default output; the other formats are one flag each: `--xml` for models that like tags, `--html` for a
standalone page, `--bare` for plain text with almost no wrapping.

## Where files come from

A URL can sit anywhere a path can, and short repository URLs are pulled down as archives, so you can bundle a project
you haven't cloned (GitHub, GitLab, Bitbucket, and Codeberg):

```bash
lx -n 10 github.com/eignex/combo
lx https://gitlab.com/owner/repo/-/tree/dev
```

Three flags reach into files that would otherwise be skipped:

```bash
lx -Z archive.zip   # expand an archive (zip, tar, 7z, rar, and friends)
lx -D docs/         # text out of PDFs, Word docs, spreadsheets, slide decks
lx -M assets/       # codec, duration, and dimensions of audio, video, and images
```

`-M` reads container headers only, so its cost doesn't grow with the length of the file.

## Stream processing

lx reads its arguments left to right, and an option only applies to the paths after it. An option (or `-s`) following a
path starts a new section with the per-section options reset, so they don't leak into the next group.

```bash
lx --tail 50 app.log \
   -u src/ \
   -i "*.md" docs/
```

That's three sections: the last 50 lines of the log, skeletons from src/, and only the Markdown under docs/. `-s` names a
group, and the name is used as the section title in XML output:

```bash
lx -s "Code under test" src/database/users \
   -s "Test fixtures"   src/tests/fixtures
```

## Configuration

Drop a `~/.config/lx/config.yaml` to change the defaults; anything you leave out keeps its built-in value.

```yaml
output_mode: "copy"    # stdout | copy
output_format: "xml"   # markdown | xml | html | bare
prompts_dir: "~/Workspaces/prompts"
```

Two ready-made profiles ship with the repo: [default_config.yaml](default_config.yaml) and
[xml_config.yaml](xml_config.yaml). Point at one for a single run with `lx -y xml_config.yaml src/`, or set
`$LX_CONFIG` to make it the default. [CONFIG.md](CONFIG.md) has the full reference, including template context and
helpers.

## Coding-agent skill

[skills/lx/SKILL.md](skills/lx/SKILL.md) teaches coding agents to reach for lx when they explore a codebase. Symlink
it wherever your agent looks for skills:

```bash
ln -s "$(pwd)/skills/lx" ~/.claude/skills/lx
ln -s "$(pwd)/skills/lx" ~/.config/opencode/skills/lx
```
