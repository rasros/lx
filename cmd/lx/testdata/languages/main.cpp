// UI widget with a single integer value.
class Widget {
public:
    int value; // current value
    void set(int v) {
        value = v;
    }
private:
    int secret; /* internal state */
    void hide();
};

/* Free function — no class needed. */
int FreeFn(int v) {
    return v;
}

// Combine two values.
int Combine(
    int a,
    int b
) {
    return a + b;
}
