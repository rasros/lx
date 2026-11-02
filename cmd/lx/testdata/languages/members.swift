public struct Grid {
    public var width: Int = 0

    public subscript(i: Int) -> Int {
        return i
    }

    private func hidden() {}
}

// An alias is a type.
public typealias Width = Int

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
