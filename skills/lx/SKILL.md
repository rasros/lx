---
name: lx
description: Codebase exploration tool that reads many files or whole directories in a single call, with per-file headers, glob include/exclude filters, function/type skeleton extraction (signatures only, no bodies), and head/tail line slicing.
---

# lx for exploration

```bash
lx -u -Y pkg/                                    # skeleton: type defs + function signatures, no bodies
lx -i '*.go' -e '*_test.go' src/                 # glob include/exclude filters
lx -n 30 src/                                    # peek at every file (15 head + 15 tail each)
lx -n 0 src/                                     # filenames + sizes only, no content
lx --tail 200 app.log                            # last 200 lines of a log
lx -l src/foo.go                                 # line numbers, for citing locations
lx -u -Y -i '*.{ts,tsx}' -e 'node_modules/' web/ # combining filters
```

Skeletons (`-u` functions, `-Y` types) cover 19 languages (Go, Python,
TypeScript, Java, Rust, C/C++, …); other file types pass through unchanged.
Find the declaration you care about, then Read its body.

`-i`/`-e` are repeatable and additive; excludes win on conflict. Patterns
match basenames and full paths; a trailing slash matches directories, so
`-e 'vendor/' -e 'node_modules/'` drops whole subtrees.

Each path becomes its own output section, and interleaved flags (`-n`,
`--head`, `--tail`, `-l`, `-i`, `-e`, `-u`, `-Y`, …) reset between sections —
put shared flags before the first path.

## Other capabilities

```bash
lx github.com/owner/repo                         # remote repo without cloning (also gitlab/bitbucket/codeberg)
git ls-files '*.go' | lx -u -Y -                 # paths from stdin (-0 for NUL-separated)
lx -D docs/                                      # extract text from PDF/DOCX/XLSX/PPTX
lx -Z -i '*.go' archive.zip                      # expand a local archive (not needed for repo URLs)
lx --stats -u -Y src/ > /dev/null                # token estimate on stderr; redirect drops the bundle
```

For anything not covered here, `lx --help` is the source of truth.
