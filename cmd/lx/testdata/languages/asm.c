static void kernel(long n, double *x)
{
    __asm__ __volatile__ (
        "vxorpd %%ymm0, %%ymm0, %%ymm0 \n\t"
        "vmulpd (%1), %%ymm1, %%ymm2 \n\t"
        : "+r" (n)
        : "r" (x)
        : "cc", "memory"
    );
    __asm__ __volatile__ ("" ::: "memory");
}

#ifdef HAVE_DOT
/* Dot product. */
double dot(long n, double *x, double *y);
#endif

/* Directives between concatenated strings are real. */
static const char *banner =
    "kernel "
#ifdef HAVE_FMA
    "fma"
#else
    "plain"
#endif
    ;
