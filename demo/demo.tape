# VHS tape for lx demo — render with: make demo
# https://github.com/charmbracelet/vhs

Output demo/demo.gif

Require lx

Set Shell "bash"
Set FontSize 18
Set Width 900
Set Height 420
Set Padding 20
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
Sleep 1.5s


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
Sleep 2.5s


Ctrl+L
Type "# Options apply to the files after them: two sections, one command."
Enter
Sleep 200ms
Type "lx -s Overview -t . -s Code -l -i '*.go' src/"
Sleep 200ms
Enter
Sleep 2.5s


Ctrl+L
Type "# And -c puts it on the clipboard."
Enter
Sleep 200ms
Type "lx -c"
Sleep 200ms
Enter
Sleep 2s
