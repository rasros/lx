// A macro call left open: error recovery leaves nodes the grammar hides
// outside any error node.
#include "set.h"
    set & set::erase() {
    }
        CHECK(
            iterator end = m_items.end();
