#ifndef SOLUTION_H
#define SOLUTION_H

// C++ in a .h file, read as C.
struct Solution {
  bool valid = false;
  std::vector<double> values;
  void clear();
  void print(const std::string& prefix = "",
             const std::string& message = "") const;
  bool isValid(int i) const {
    return i < 3;
  }
};

int solve(struct Solution *s);

#endif
