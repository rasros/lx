export class Greeter {
    message = "hi";

    greet(name) {
        return this.message + " " + name;
    }
}

export function topLevel(x) {
    return x + 1;
}
