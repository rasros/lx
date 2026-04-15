module Demo where

-- | Animals supported by the system.
data Animal = Dog | Cat

{- A newtype wrapper for names.
   Avoids mixing up plain strings. -}
newtype Name = Name String

-- | Greet by name, with a fallback.
greet name
  | name == "" = "Hello!"
  | otherwise  = "Hello, " ++ name -- append name

-- | Add two integers.
add x y
  | x >= 0    = x + y
  | otherwise = y
