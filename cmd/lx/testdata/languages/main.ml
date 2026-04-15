(* Basic animal variants. *)
type animal = Dog | Cat

(* A 2-D point record.
   Both fields are floats. *)
type point = {
  x: float;
  y: float;
}

(* Greet by prepending "Hello, ". *)
let greet name =
  "Hello, " ^ name

(* Combine name and greeting
   with a separator. *)
let combine
    name
    greeting =
  greeting ^ ", " ^ name
