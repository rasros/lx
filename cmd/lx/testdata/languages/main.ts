export interface Shape {
    area(): number;
    name: string;
}

export class Point {
    public x: number;
    public y: number;
    private label: string;

    constructor(x: number, y: number) {
        this.x = x;
        this.y = y;
        this.label = "";
    }

    public scale(factor: number): Point {
        return new Point(this.x * factor, this.y * factor);
    }

    private helper(): void {}
}

export function topLevel(x: number): number {
    return x + 1;
}

export function combine(
    x: number,
    y: number,
): number {
    return x + y;
}
