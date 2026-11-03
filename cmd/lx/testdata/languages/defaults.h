// C++ default arguments in a header read as C: still functions.
void render(int x, int y = 0);
int scale(int v, int factor = 2);

// Initialized variables, a function pointer among them: not functions.
int (*on_draw)(int) = 0;
int frames = 60;

// A pure virtual read as C: the `= 0` reads as an initializer.
virtual char const * get_name() const = 0;
