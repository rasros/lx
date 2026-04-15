/// An animal with a name and species.
class Animal {
  final String name;
  final String species; // taxonomy

  Animal(this.name, this.species);

  // Make the animal speak.
  String speak() {
    return '...';
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
