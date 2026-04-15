<?php
interface Shape {
    public function area(): float;
    public function perimeter(): float;
}

class Point {
    public float $x;
    public float $y;
    private string $label;

    public function __construct(float $x, float $y) {
        $this->x = $x;
        $this->y = $y;
    }

    public function scale(float $factor): Point {
        return new Point($this->x * $factor, $this->y * $factor);
    }

    private function helper(): void {}
}

function topLevel(int $x): int {
    return $x + 1;
}

function combine(
    int $x,
    int $y,
): int {
    return $x + $y;
}
