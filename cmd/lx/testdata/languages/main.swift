public protocol Shape {
    func area() -> Double
    func perimeter() -> Double
}

public struct Point {
    public var x: Double
    public var y: Double
    private var label: String

    public init(x: Double, y: Double) {
        self.x = x
        self.y = y
        self.label = ""
    }

    public func scale(factor: Double) -> Point {
        return Point(x: x * factor, y: y * factor, label: label)
    }

    private func helper() -> Int { 42 }
}

public func topLevel(x: Double) -> Double {
    return x + 1
}
