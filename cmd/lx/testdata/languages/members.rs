use std::fmt;

pub struct Meters(f64);

/// Distances in meters: an alias is a type.
pub type Distance = Meters;

impl Meters {
    pub fn new(v: f64) -> Self {
        Meters(v)
    }

    fn private_helper(&self) {}
}

// Trait impl items are public through the trait.
impl fmt::Display for Meters {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        write!(f, "{}m", self.0)
    }
}
