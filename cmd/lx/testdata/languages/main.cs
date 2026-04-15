public interface IShape {
    double Area();
    double Perimeter();
}

public class Point {
    public double X { get; set; }
    public double Y { get; set; }
    private string label;

    public Point(double x, double y) {
        X = x;
        Y = y;
    }

    public Point Scale(double factor) {
        return new Point(X * factor, Y * factor);
    }

    private void Helper() {}
}
