/* A C++ header with only methods and a reference: the C grammar fails
   on it and the C++ grammar parses it whole. */
struct Buffer {
    int len() const;
    void set(int &v);
};
