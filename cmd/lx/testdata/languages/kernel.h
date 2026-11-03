/* A C header the C grammar fails on (the annotations after
   the lock functions' parameters)
   and the C++ grammar does not. Its asm clobbers and attribute read like
   C++ to a text scan, but nothing in it is C++, so it stays C. */
#ifndef KERNEL_H
#define KERNEL_H

struct device_class;

static inline void barrier(void) { asm volatile("" ::: "memory"); }

[[gnu::always_inline]] static inline int get(struct device_class *class) { return 1; }

#ifdef CONFIG_DEBUG_LOCK
extern void lock(lock_t *lock) __acquires(lock);
extern void unlock(lock_t *lock) __releases(lock);
#else
# define lock(l) ((void)(l))
#endif

struct page {
    struct page *new;
    int class;
};

#endif
