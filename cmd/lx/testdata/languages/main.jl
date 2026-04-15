# Animal struct with name and species.
struct Animal
    name::String
    species::String  # taxonomy
end

#= Abstract base for all shapes.
   Subtypes must implement area(). =#
abstract type Shape end

# Greet someone by name.
function greet(name::String)
    return "Hello, " * name
end

# Combine name and greeting.
function combine(
    name::String,
    greeting::String,
)
    return greeting * ", " * name
end
