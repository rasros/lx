namespace geo {

// A digit separator opens no character literal.
int scale(int x) {
    for (int i = 0; i < 10'000; ++i) {
        x += i;
    }
    return x;
}

}  // namespace geo
