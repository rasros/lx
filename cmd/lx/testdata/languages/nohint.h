/* A C++ header with none of the words that hint at C++: the C grammar
   fails on it and the C++ grammar parses it whole. */
struct Buffer {
    int len() const;
    void set(int &v);
};
