# VHS tape for lx demo — render with: make demo
# https://github.com/charmbracelet/vhs

Output demo/demo.gif

Require lx

Set Shell "bash"
Set FontSize 20
Set Width 920
Set Height 600
Set Theme "Catppuccin Mocha"
Set TypingSpeed 60ms
Set PlaybackSpeed 1.0

# Spin up a self-contained sandbox project (see demo/setup.sh).
Hide
Type "source demo/setup.sh"
Enter
Sleep 400ms
Ctrl+L
Show


Type "# Bundle current dir as Markdown."
Enter
Sleep 300ms
Type "lx"
Sleep 300ms
Enter
Sleep 2s


Ctrl+L
Type "# -c copies straight to the system clipboard."
Enter
Sleep 400ms
Type "lx -c"
Sleep 400ms
Enter
Sleep 2.5s


Ctrl+L
Type "# Just the file tree."
Enter
Sleep 300ms
Type "lx -t"
Sleep 300ms
Enter
Sleep 2.5s


Ctrl+L
Type "# Function signatures and types of go files."
Enter
Sleep 300ms
Type "lx -Y -i '*.go' src/"
Sleep 300ms
Enter
Sleep 3s


Ctrl+L
Type "# XML output for Claude-style prompts."
Enter
Sleep 400ms
Type "lx --xml src/"
Sleep 400ms
Enter
Sleep 3s


Ctrl+L
Type "# Two sections in one command:"
Enter
Type "# Tree first, then -l numbered Go files."
Enter
Sleep 400ms
Type "lx -s 'Tree overview' -t . -s Code -l -i '*.go' src/"
Sleep 400ms
Enter
Sleep 3s


Ctrl+L
Type "# -P loads a prompt from your library ($LX_PROMPTS_DIR)."
Enter
Type "# It resolves go/test -> prompts/go/test.md and prepends."
Enter
Sleep 400ms
Type "lx -P go/test src/"
Sleep 400ms
Enter
Sleep 3s
