/// Mixes in walking.
mixin Walker {
  int steps = 0;

  void walk() {
    steps++;
  }
}

extension Shout on String {
  String shout() {
    return toUpperCase();
  }
}

enum Size {
  small,
  large;

  bool get isLarge {
    return this == large;
  }
}
