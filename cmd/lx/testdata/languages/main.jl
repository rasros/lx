struct Animal
    name::String
    species::String
end

abstract type Shape end

function greet(name::String)
    return "Hello, " * name
end

function combine(
    name::String,
    greeting::String,
)
    return greeting * ", " * name
end
