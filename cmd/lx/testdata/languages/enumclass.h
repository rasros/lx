/* A C++ header without classes, namespaces or templates. */
#pragma once

enum class Color { Red, Green };

struct Point {
    int x = 0;
    int y = 0;

    /* Length along both axes. */
    int manhattan() const {
        if (x < 0) {
            return -x + (y < 0 ? -y : y);
        }
        return x + (y < 0 ? -y : y);
    }
    bool operator==(const Point& other) const { return x == other.x && y == other.y; }
};

Point origin();
