/* blockdoc.c: block comments above declarations. */
#include <stddef.h>

int helper(void) { return 1; }

/**
   A block doc whose lines do not start with an asterisk: kept whole above
   the function it documents.
*/
int documented(void);

/*
   Unstarred lines that read like code stay in the doc:
   Example: x = 1;
   #define-like text
*/
int example(void);

// A line comment ending in a block closer, globs like **/
int globbed(void);

/* The tails of trailing field comments document nothing below them. */
struct irq {
    int vcpu;       /* The VCPU it is queued on,
                     * or none.
                     */
    int target;     /* The VCPU it should go to,
                     * read by the guest. */
    int pending;
};
