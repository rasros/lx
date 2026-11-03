/* A C header the C++ grammar empties. It reads the comparison below as a
   template call, which is C++, but loses both functions. */
static const int ordered = one < two > (three);

int lsame(const char* ca, const char* cb
#ifndef NO_LENGTHS
          , int lca, int lcb
#endif
);

/* Returns the norm of x. */
double nrm2(int n, const double* x);
