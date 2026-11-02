#ifndef WIDGET_H
#define WIDGET_H

#include <string>

namespace ui {

// C++ in a .h file: parsed as C++.
class Widget {
public:
    virtual ~Widget() = default;
    // Pure virtual.
    virtual void draw(int x, int y) = 0;
    virtual std::string get_name() const = 0;
    void resize(int width, int height = 1);
    int id() const {
        return id_;
    }

private:
    int id_;
};

void render(Widget &w, bool force = false);

}  // namespace ui

#endif
