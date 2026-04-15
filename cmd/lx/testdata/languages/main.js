export class Greeter {
    message = "hi";

    greet(name) {
        return this.message + " " + name;
    }
}

export function topLevel(x) {
    return x + 1;
}

export function combine(
    x,
    y,
) {
    return x + y;
}
