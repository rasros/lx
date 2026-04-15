/** A simple greeter class. */
export class Greeter {
    message = "hi"; // default message

    // Greet by name.
    greet(name) {
        return this.message + " " + name;
    }
}

/* Top-level utility function. */
export function topLevel(x) {
    return x + 1;
}

// Combine two values.
export function combine(
    x,
    y,
) {
    return x + y;
}
