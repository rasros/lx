#include <vector>

// Exported with C linkage.
extern "C" int c_entry(int argc);

extern "C" {
// Opaque handle for C callers.
struct Handle {
    int id;
};

int handle_open(Handle *h);

typedef void (*callback_t)(Handle *h);
}

// Aliases are types.
using Id = unsigned long;
template <typename T>
using Vec = std::vector<T>;

// Pointer to a function: a variable.
int (*dispatch)(int) = nullptr;

#if defined(DEBUG)
void trace(const char *msg) {
    (void)msg;
}
#endif

int run(int argc) {
    return c_entry(argc);
}
