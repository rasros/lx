/// A registered user with a public name.
pub struct User {
    pub name: String,
    email: String, // kept private
}

// Possible roles for a user.
pub enum Role {
    Admin,
    User,
}

/// Greeting behaviour.
pub trait Greet {
    fn greet(&self) -> String;
}

/* User method implementations. */
impl User {
    pub fn new(name: String) -> Self {
        User { name, email: String::new() }
    }

    pub fn display(&self) -> String {
        self.name.clone()
    }

    fn helper(&self) {} // private
}

/// Create a User from a name alone.
pub fn create(name: String) -> User {
    User::new(name)
}

/// Create a User with both fields.
pub fn combine(
    name: String,
    email: String,
) -> User {
    User { name, email }
}

fn private_fn() {}
