<?php
/** Geometric shape contract. */
interface Shape {
    public function area(): float;
    public function perimeter(): float;
}

/**
 * Immutable 2-D point.
 */
class Point {
    public float $x;
    public float $y;
    private string $label; // internal

    # Construct from coordinates.
    public function __construct(float $x, float $y) {
        $this->x = $x;
        $this->y = $y;
    }

    /* Scale this point. */
    public function scale(float $factor): Point {
        return new Point($this->x * $factor, $this->y * $factor);
    }

    private function helper(): void {}
}

// Simple increment.
function topLevel(int $x): int {
    return $x + 1;
}

# Combine two integers.
function combine(
    int $x,
    int $y,
): int {
    return $x + $y;
}
