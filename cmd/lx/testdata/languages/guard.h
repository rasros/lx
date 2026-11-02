#ifndef GUARD_H
#define GUARD_H

/* Declared before its definition. */
struct fwd;

/* A struct and a variable of it in one declaration. */
struct settings {
    int verbose;
    int level;
} default_settings;

/* Variables, initialized or not, are no functions. */
static int (*on_exit_hook)(int) = 0;
static int counter = 1;

#if defined(USE_THREADS) && \
    defined(HAVE_PTHREAD)
int lock_init(void);
#elif defined(_WIN32)
int lock_init_win(void);
#else
/* No locking. */
#endif

int guard_version(void);

/* One body, two signatures. */
#ifdef OLD_API
static inline int open_port(int port, int flags)
#else
static inline int open_port(int port)
#endif
{
    return port;
}

#endif /* GUARD_H */
