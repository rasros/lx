// C++ default arguments in a header read as C: still functions.
void render(int x, int y = 0);
int scale(int v, int factor = 2);

// Initialized variables, a function pointer among them: not functions.
int (*on_draw)(int) = 0;
int frames = 60;
