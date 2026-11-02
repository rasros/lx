#ifndef TOPLEVEL_H
#define TOPLEVEL_H

#ifdef __cplusplus
extern "C" {
#endif

/* Plain struct, not a typedef. */
struct buffer {
    char *data;
    int (*grow)(struct buffer *b, int n); /* function pointer field */
};

enum mode { MODE_READ, MODE_WRITE };

typedef enum {
    LEVEL_LOW,
    LEVEL_HIGH
} level;

// Callback type: a type alias, not a function.
typedef int (*handler_fn)(int code);
typedef unsigned long size_type;

// Open a buffer.
struct buffer *buffer_open(const char *path);

// Pointer to a function: a variable.
extern int (*on_error)(int code);
static void (*hooks[4])(void);

// Returns a pointer to a function: a function.
int (*pick_handler(int kind))(int code);

#ifdef _WIN32
int win_only(void);
#else
int posix_only(void);
#endif

#ifdef __cplusplus
}
#endif

#endif
