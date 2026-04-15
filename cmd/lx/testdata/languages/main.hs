module Demo where

data Animal = Dog | Cat

newtype Name = Name String

greet name
  | name == "" = "Hello!"
  | otherwise  = "Hello, " ++ name

add x y
  | x >= 0    = x + y
  | otherwise = y
