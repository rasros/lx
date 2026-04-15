/**
 * Simple calculator with a running value.
 */
public class Calculator {
    private int value;
    public static final int MAX = 100; // upper bound

    /** Initialise with a starting value. */
    public Calculator(int initial) {
        this.value = initial;
    }

    // Add x to the current value.
    public int add(int x) {
        return value + x;
    }

    /**
     * Combine value with two operands.
     *
     * @param x first operand
     * @param y second operand
     */
    public int combine(
        int x,
        int y
    ) {
        return value + x + y;
    }

    /* private helper */
    private int secret() {
        return 42;
    }
}
