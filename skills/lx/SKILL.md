---
name: lx
description: Codebase exploration tool that reads many files or whole directories in a single call, with per-file headers, glob include/exclude filters, function/type skeleton extraction (signatures only, no bodies), and head/tail line slicing.
---

# lx for exploration

```bash
lx -i '*.go' -e '*_test.go' src/                 # non-test Go files only
lx -u -Y pkg/                                    # full skeleton: type defs + function signatures
lx -Y pkg/                                       # type definitions only (with field declarations)
lx -u src/handler/auth.go                        # function signatures only
lx -n 30 src/                                    # peek at every file (15 head + 15 tail each)
lx -n 0 src/                                     # filename + size only, no content
lx --tail 200 app.log                            # last 200 lines of a log
lx github.com/owner/repo                         # output a remote repo without cloning
lx -l src/foo.go                                 # line numbers, for citing locations
lx -u -Y -i '*.{ts,tsx}' -e 'node_modules/' web/ # combining filters
```

`-u -Y` keeps function signatures and type definitions, drops bodies. Find the
function you care about, then Read its body. Supported: C, C++, C#, Dart, Go,
Groovy, Haskell, Java, Kotlin, Objective-C, OCaml, PHP, Python, Ruby, Rust,
Scala, Swift, TypeScript, Zig. Other file types pass through unchanged.

`-i '<glob>'` includes, `-e '<glob>'` excludes. Both are repeatable and
additive: a file is kept if it matches **any** include and is dropped if it
matches **any** exclude (excludes win on conflict). When no `-i` is given,
everything is included by default. Patterns can be globs (`*.go`,
`*.{js,ts}`), bare names (`vendor`), or directory-only with a trailing slash
(`vendor/`, `node_modules/`), and they match against both basenames and full
paths. So `-e 'vendor/' -e 'node_modules/'` is the idiomatic way to drop
whole subtrees.

Short repo URLs (`github.com/...`, `gitlab.com/...`, `bitbucket.org/...`,
`codeberg.org/...`) are auto-rewritten to the archive zip and expanded; no
explicit `-Z` needed.

## Sections and flag scoping

Each path becomes its own output section with a path header. Interleaved
flags (`-n`, `--head`, `--tail`, `-l`, `-i`, `-e`, `-u`, `-Y`, `-D`, `-Z`,
`-H`, `-I`) reset between sections, so put shared flags before the first
path: `lx -l -n 30 file1 file2`.

## Other capabilities

```bash
git ls-files '*.go' | lx -u -Y -                # paths from stdin (-0 for NUL-separated)
lx -D docs/                                     # extract text from PDF/DOCX/XLSX/PPTX
lx -Z -i '*.go' archive.zip                     # expand a local archive (not needed for repo URLs)
lx --stats -u -Y src/ > /dev/null               # token estimate on stderr; redirect drops the bundle
lx -c src/                                      # copy bundle to clipboard instead of stdout
```

For anything not covered here, `lx --help` is the source of truth — flags
are grouped Global / Interleaved / Action.
