namespace Shapes
{
    /// <summary>A shape.</summary>
    public class Shape
    {
        int implicitlyPrivate;
        void ImplicitlyPrivateMethod() {}
        public int Sides { get; set; }
        public event EventHandler Changed;
        public delegate void Handler(int x);
        public Shape(int sides)
        {
            Sides = sides;
        }
        ~Shape()
        {
        }
        public int this[int i]
        {
            get { return i; }
        }
        public static Shape operator +(Shape a, Shape b)
        {
            return a;
        }
        public static implicit operator int(Shape s)
        {
            return s.Sides;
        }
        protected internal void Shared()
        {
        }
        private void Hidden()
        {
        }
        class ImplicitlyPrivateNested
        {
            public void N() {}
        }
    }

    public interface IArea
    {
        double Area();
    }

    public enum Kind
    {
        Square,
        Circle,
    }
}
