public struct Grid {
    public var width: Int = 0

    public subscript(i: Int) -> Int {
        return i
    }

    private func hidden() {}
}

public class Resource {
    deinit {
        print("bye")
    }
}

public enum Direction {
    case north
    case south

    public func flipped() -> Direction {
        return self
    }
}
