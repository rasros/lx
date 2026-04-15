package demo

// Person represents a named individual.
type Person struct {
	Name string
	age  int // unexported
}

/* Worker defines the work contract. */
type Worker interface {
	Work()
	rest()
}

// NewPerson creates a Person with the given name.
func NewPerson(name string) Person {
	return Person{Name: name}
}

// NewWorker creates a worker with name and role.
func NewWorker(
	name string,
	role string,
) Person {
	return Person{Name: name}
}

func (p Person) Greet() string {
	return p.Name
}

func helper() {} // unexported helper
