(* A constant, not a function. *)
let pi = 3.14159

(* A function with parameters. *)
let area r =
  pi *. r *. r

let double = fun x -> 2 * x

let classify = function
  | 0 -> "zero"
  | _ -> "other"

module Config = struct
  (* A module value: a field. *)
  let default_size = 10

  let scale x =
    x * default_size
end
