package demo

type Person struct {
	Name string
	age  int
}

type Worker interface {
	Work()
	rest()
}

func NewPerson(name string) Person {
	return Person{Name: name}
}

func NewWorker(
	name string,
	role string,
) Person {
	return Person{Name: name}
}

func (p Person) Greet() string {
	return p.Name
}

func helper() {}
