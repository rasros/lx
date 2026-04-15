/** Geometric shape contract. */
export interface Shape {
    area(): number;
    name: string; // display label
}

/**
 * Immutable 2-D point.
 */
export class Point {
    public x: number;
    public y: number;
    private label: string; /* internal */

    constructor(x: number, y: number) {
        this.x = x;
        this.y = y;
        this.label = "";
    }

    // Scale this point by a factor.
    public scale(factor: number): Point {
        return new Point(this.x * factor, this.y * factor);
    }

    private helper(): void {}
}

// Top-level utility.
export function topLevel(x: number): number {
    return x + 1;
}

/** Combine two numbers. */
export function combine(
    x: number,
    y: number,
): number {
    return x + y;
}
