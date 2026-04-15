/// An animal with a name and species.
class Animal {
  final String name;
  final String species; // taxonomy
  String _tag; // internal

  Animal(this.name, this.species);

  // Make the animal speak.
  String speak() {
    return '...';
  }

  void _helper() { // internal utility
    return;
  }
}

/* Greet a person by name. */
String greet(String name) {
  return 'Hello, $name';
}

/// Combine name and greeting.
String combine(
  String name,
  String greeting,
) {
  return '$greeting, $name';
}
