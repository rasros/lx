#include <vector>

// Widget holds a value.
class Widget {
    int implicitly_private;
public:
    Widget();
    virtual ~Widget();
    // Nested type: walked with its own (struct, public) access.
    struct Inner {
        int a;
        void m();
    };
    class Deep {
        int hidden;
    public:
        void d();
    };
    void (*callback)(int);
    int plain;
    template <typename T>
    T get(T x) {
        return x;
    }
    static int count();
    operator bool() const;
    friend bool operator==(const Widget& a, const Widget& b);
    friend class WidgetFactory;
#ifdef WIDGET_DEBUG
    void dump() const;
#endif
private:
    int secret;
    void hide();
protected:
    void prot();
};

enum class Color {
    Red,
    Green,
};

namespace geo {
// Point in the plane.
struct Point {
    double x;
    double y;
    double norm() const;
};
void free_fn();
}

template <typename T>
class Box {
public:
    T get();
};

template <typename T>
T identity(T x) {
    return x;
}
