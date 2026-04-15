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

(* Module for geometry utilities. *)
module Geometry = struct
  type shape = Circle | Square

  (* Area of a unit circle. *)
  let pi = 3.14159

  let area radius =
    pi *. radius *. radius
end

(* Signature for printable types. *)
module type Printable = sig
  val to_string : 'a -> string
end
