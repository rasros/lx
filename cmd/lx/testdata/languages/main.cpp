class Widget {
public:
    int value;
    void set(int v) {
        value = v;
    }
private:
    int secret;
    void hide();
};

int FreeFn(int v) {
    return v;
}

int Combine(
    int a,
    int b
) {
    return a + b;
}
