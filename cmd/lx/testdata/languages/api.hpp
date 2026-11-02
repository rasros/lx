#ifndef API_HPP
#define API_HPP

#ifdef __cplusplus
extern "C" {
#endif

/* Operations table. */
struct ops {
    int (*run)(int code);
};

int afunc(void);

enum color { RED, GREEN };

#ifdef __cplusplus
}
#endif

#endif
