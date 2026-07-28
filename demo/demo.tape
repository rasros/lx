# VHS tape for lx demo — render with: make demo
# https://github.com/charmbracelet/vhs

Output demo/demo.gif

Require lx

Set Shell "bash"
Set FontSize 18
Set Width 900
Set Height 420
Set Padding 10
Set Theme "Catppuccin Mocha"
Set TypingSpeed 40ms
Set PlaybackSpeed 1.0

# Spin up a self-contained sandbox project (see demo/setup.sh).
Hide
Type "source demo/setup.sh"
Enter
Sleep 400ms
Ctrl+L
Show


Type "# Bundle the current directory as Markdown."
Enter
Sleep 200ms
Type "lx"
Sleep 200ms
Enter
Sleep 3s


Ctrl+L
Type "# Just the tree."
Enter
Sleep 200ms
Type "lx -t"
Sleep 200ms
Enter
Sleep 2s


Ctrl+L
Type "# Signatures and types instead of whole bodies."
Enter
Sleep 200ms
Type "lx -u -Y src/greet.go"
Sleep 200ms
Enter
Sleep 4s


Ctrl+L
Type "# A URL works wherever a path does."
Enter
Sleep 1.5s
Type "lx -n 8 https://raw.githubusercontent.com/rasros/vectrify/main/README.md"
Sleep 400ms
Enter
Sleep 4s


Ctrl+L
Type "# And -c puts it on the clipboard."
Enter
Sleep 200ms
Type "lx -c"
Sleep 200ms
Enter
Sleep 2s
