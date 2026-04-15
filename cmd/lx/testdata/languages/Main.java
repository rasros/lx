public class Calculator {
    private int value;
    public static final int MAX = 100;

    public Calculator(int initial) {
        this.value = initial;
    }

    public int add(int x) {
        return value + x;
    }

    public int combine(
        int x,
        int y
    ) {
        return value + x + y;
    }

    private int secret() {
        return 42;
    }
}
