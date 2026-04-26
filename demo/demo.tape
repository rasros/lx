# VHS tape for lx demo — render with: make demo
# https://github.com/charmbracelet/vhs

Output demo/demo.gif

Require lx

Set Shell "bash"
Set FontSize 20
Set Width 920
Set Height 600
Set Theme "Catppuccin Mocha"
Set TypingSpeed 90ms
Set PlaybackSpeed 1.0

# Spin up a self-contained sandbox project (see demo/setup.sh).
Hide
Type "source demo/setup.sh"
Enter
Sleep 600ms
Ctrl+L
Show

# 1. Basic bundle: walk the current directory, print markdown.
Type "lx"
Sleep 800ms
Enter
Sleep 2.5s

# 2. Tree view of the project structure.
Ctrl+L
Type "lx -t"
Sleep 500ms
Enter
Sleep 2s

# 3. Code skeleton — signatures only, no bodies.
Ctrl+L
Type "lx -Y -i '*.go' src/"
Sleep 500ms
Enter
Sleep 2.5s

# 4. XML output for Claude-style prompts.
Ctrl+L
Type "lx --xml src/"
Sleep 500ms
Enter
Sleep 2.5s

# 5. Mix sections: tree of the repo, then only Go files with line numbers.
Ctrl+L
Type "lx -s 'Tree overview' -t . -s Code -l -i '*.go' src/"
Sleep 500ms
Enter
Sleep 3s

# 6. Pipe-friendly: copy straight to clipboard for LLM paste.
Ctrl+L
Type "# Paste straight into Claude / ChatGPT:"
Enter
Type "lx -c"
Sleep 500ms
Enter
Sleep 2.2s
