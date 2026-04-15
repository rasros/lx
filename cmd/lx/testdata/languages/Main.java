public class Calculator {
    private int value;
    public static final int MAX = 100;

    public Calculator(int initial) {
        this.value = initial;
    }

    public int add(int x) {
        return value + x;
    }

    private int secret() {
        return 42;
    }
}
