greet() {
    local name=$1
    echo "Hello, $name"
}

add() {
    local a=$1
    local b=$2
    echo $(($a + $b))
}

_helper() {
    echo "internal"
}
