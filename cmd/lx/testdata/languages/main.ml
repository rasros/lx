type animal = Dog | Cat

type point = {
  x: float;
  y: float;
}

let greet name =
  "Hello, " ^ name

let combine
    name
    greeting =
  greeting ^ ", " ^ name
