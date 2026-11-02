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

export enum Kind {
    Square,
    Circle = 2,
}
