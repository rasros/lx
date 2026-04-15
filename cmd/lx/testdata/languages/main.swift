/// Geometric shape contract.
public protocol Shape {
    func area() -> Double
    func perimeter() -> Double
}

/**
 * Immutable 2-D point with label.
 */
public struct Point {
    public var x: Double
    public var y: Double
    private var label: String // internal

    /// Initialise from coordinates.
    public init(x: Double, y: Double) {
        self.x = x
        self.y = y
        self.label = ""
    }

    // Scale the point by a factor.
    public func scale(factor: Double) -> Point {
        return Point(x: x * factor, y: y * factor, label: label)
    }

    private func helper() -> Int { 42 } /* unused */
}

/// Top-level increment function.
public func topLevel(x: Double) -> Double {
    return x + 1
}

// Combine two doubles.
public func combine(
    x: Double,
    y: Double,
) -> Double {
    return x + y
}
