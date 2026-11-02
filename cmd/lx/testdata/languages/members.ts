/** Base shape. */
export abstract class Shape {
    #secret = 1;
    private hidden = 2;
    protected sides = 3;

    abstract area(): number;

    describe(): string {
        return "shape";
    }

    #helper(): void {}
}

/** Sides keyed by kind: an alias is a type. */
export type Sides = { [k in Kind]: number };

export enum Kind {
    Square,
    Circle = 2,
}
