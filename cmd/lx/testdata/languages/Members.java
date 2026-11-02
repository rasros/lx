package p;

/** Color enum. */
public enum Color {
    RED,
    GREEN;

    /** Hex value. */
    public String hex() {
        return "x";
    }

    private int ordinalSquared() {
        return 0;
    }
}

public record Pt(int x, int y) {
    public Pt {
        if (x < 0) throw new IllegalArgumentException();
    }

    public int sum() {
        return x + y;
    }
}

public @interface Marker {
    String value() default "";
}

public interface Consts {
    int MAX = 10;

    default int twice(int a) {
        return a * 2;
    }
}
