/* 2-D point type */
typedef struct Point {
    int x; /* x coordinate */
    int y; /* y coordinate */
} Point;

// Add two integers and return the sum.
int add(int a, int b) {
    return a + b;
}

/* Subtract b from a over
   multiple lines. */
int subtract(
    int a,
    int b
) {
    return a - b;
}

void greet(const char *name); // forward declaration
