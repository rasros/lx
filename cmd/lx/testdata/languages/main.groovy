/** Animal with name and species. */
class Animal {
    String name
    String species // taxonomy

    // Make the animal speak.
    def speak() {
        return "..."
    }

    /* private helper method */
    private def helper() {
        return 0
    }
}

// Standalone increment.
def standalone(x) {
    return x + 1
}

/* Combine two values. */
def combine(
    x,
    y) {
    return x + y
}
