class Animal {
  final String name;
  final String species;

  Animal(this.name, this.species);

  String speak() {
    return '...';
  }
}

String greet(String name) {
  return 'Hello, $name';
}

String combine(
  String name,
  String greeting,
) {
  return '$greeting, $name';
}
