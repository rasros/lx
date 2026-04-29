# VHS tape for lx demo — render with: make demo
# https://github.com/charmbracelet/vhs

Output demo/demo.gif

Require lx

Set Shell "bash"
Set FontSize 20
Set Width 920
Set Height 600
Set Theme "Catppuccin Mocha"
Set TypingSpeed 80ms
Set PlaybackSpeed 1.0

# Spin up a self-contained sandbox project (see demo/setup.sh).
Hide
Type "source demo/setup.sh"
Enter
Sleep 600ms
Ctrl+L
Show

# 1. Basic bundle: walk the cwd, emit Markdown for an LLM.
Type "# Bundle the current directory as Markdown."
Enter
Sleep 600ms
Type "lx"
Sleep 600ms
Enter
Sleep 4s

# 2. Tree view of the project structure.
Ctrl+L
Type "# Just the file tree — useful as a project overview."
Enter
Sleep 600ms
Type "lx -t"
Sleep 600ms
Enter
Sleep 3.5s

# 3. Code skeleton — signatures only, no bodies.
Ctrl+L
Type "# Signatures and types only (skeleton mode), great for big repos."
Enter
Sleep 600ms
Type "lx -Y -i '*.go' src/"
Sleep 600ms
Enter
Sleep 4s

# 4. XML output for Claude-style prompts.
Ctrl+L
Type "# XML output — Claude prefers this structure."
Enter
Sleep 600ms
Type "lx --xml src/"
Sleep 600ms
Enter
Sleep 4s

# 5. Mix sections: tree of the repo, then only Go files with line numbers.
Ctrl+L
Type "# Two sections in one command: tree first, then -l numbered Go files."
Enter
Sleep 600ms
Type "lx -s 'Tree overview' -t . -s Code -l -i '*.go' src/"
Sleep 600ms
Enter
Sleep 5s

# 6. Reusable prompts from a directory ($LX_PROMPTS_DIR).
Ctrl+L
Type "# -P loads a prompt from your library ($LX_PROMPTS_DIR)."
Enter
Sleep 600ms
Type "# It resolves go/test -> prompts/go/test.md and prepends it to the bundle."
Enter
Sleep 600ms
Type "lx -P go/test src/"
Sleep 600ms
Enter
Sleep 5s

# 7. Pipe-friendly: copy straight to clipboard for LLM paste.
Ctrl+L
Type "# -c copies the rendered bundle straight to the system clipboard."
Enter
Sleep 600ms
Type "lx -c"
Sleep 600ms
Enter
Sleep 3s
