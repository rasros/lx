# Greet someone by name.
greet() {
    local name=$1
    echo "Hello, $name"
}

# Add two numbers.
add() {
    local a=$1
    local b=$2
    echo $(($a + $b))
}

_helper() { # internal
    echo "internal"
}
