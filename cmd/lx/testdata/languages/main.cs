/// <summary>Geometric shape contract.</summary>
public interface IShape {
    double Area();
    double Perimeter();
}

/**
 * Immutable 2-D point.
 */
public class Point {
    public double X { get; set; }
    public double Y { get; set; }
    private string label; /* internal */

    // Construct from coordinates.
    public Point(double x, double y) {
        X = x;
        Y = y;
    }

    /// <summary>Scale this point.</summary>
    public Point Scale(double factor) {
        return new Point(X * factor, Y * factor);
    }

    // Translate by deltas.
    public Point Combine(
        double dx,
        double dy
    ) {
        return new Point(X + dx, Y + dy);
    }

    private void Helper() {}
}
