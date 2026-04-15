# Animal with a name and a maximum count.
class Animal
  attr_reader :name
  MAX = 100 # class constant

  def initialize(name)
    @name = name
  end

  # Make the animal speak.
  def speak
    "..."
  end

  private

  def helper
    42
  end
end

=begin
Standalone utility functions
below the class definition.
=end

def standalone(x)
  x + 1
end

# Combine two values.
def combine(
  x,
  y
)
  x + y
end
