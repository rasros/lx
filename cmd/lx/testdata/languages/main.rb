class Animal
  attr_reader :name
  MAX = 100

  def initialize(name)
    @name = name
  end

  def speak
    "..."
  end

  private

  def helper
    42
  end
end

def standalone(x)
  x + 1
end

def combine(
  x,
  y
)
  x + y
end
