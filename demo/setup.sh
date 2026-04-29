# Sandbox project used by demo/demo.tape. Source this: `source demo/setup.sh`
# It creates a throwaway dir, populates a tiny Go project, and cds into it.
_lx_demo_dir="$(mktemp -d)"
mkdir -p "$_lx_demo_dir/src"

cat >"$_lx_demo_dir/src/main.go" <<'EOF'
// Package main is the demo entrypoint used by lx's VHS tape.
package main

import "fmt"

// main wires up a Greeter and prints a single greeting so the demo has
// something concrete to bundle.
func main() {
	g := Greeter{Prefix: "hello"}
	fmt.Println(g.Greet("world"))
}
EOF

cat >"$_lx_demo_dir/src/greet.go" <<'EOF'
package main

// Greeter builds greeting strings with a configurable prefix.
type Greeter struct {
	// Prefix is prepended to every name passed to Greet.
	Prefix string
}

// Speaker is anything that can produce a greeting for a given name.
type Speaker interface {
	// Greet returns a greeting addressed to name.
	Greet(name string) string
}

// Greet returns "<Prefix>, <name>" using the receiver's configured prefix.
func (g Greeter) Greet(name string) string {
	return g.Prefix + ", " + name
}

// NewGreeter returns a Greeter that uses the given prefix for every greeting.
func NewGreeter(prefix string) *Greeter {
	return &Greeter{Prefix: prefix}
}
EOF

cat >"$_lx_demo_dir/README.md" <<'EOF'
# demo

A tiny project to show off lx.
EOF

mkdir -p "$_lx_demo_dir/prompts/go"
cat >"$_lx_demo_dir/prompts/go/test.md" <<'EOF'
# Go test prompt
Write table-driven tests for the Greeter type.
EOF
cat >"$_lx_demo_dir/prompts/refactor.md" <<'EOF'
# Refactor prompt
Refactor for readability without changing behavior.
EOF

export LX_PROMPTS_DIR="$_lx_demo_dir/prompts"

cd "$_lx_demo_dir"
unset _lx_demo_dir

# Pace lx's stdout for the demo so output scrolls visibly instead of painting
# the whole screen in a single frame. Terminal clipboard flag (-c) still works
# because clipboard writes bypass stdout.
lx() {
	command lx "$@" | awk '{ print; fflush(); system("sleep 0.02") }'
}
