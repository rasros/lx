pub struct User {
    pub name: String,
    email: String,
}

pub enum Role {
    Admin,
    User,
}

pub trait Greet {
    fn greet(&self) -> String;
}

impl User {
    pub fn new(name: String) -> Self {
        User { name, email: String::new() }
    }

    pub fn display(&self) -> String {
        self.name.clone()
    }

    fn helper(&self) {}
}

pub fn create(name: String) -> User {
    User::new(name)
}

pub fn combine(
    name: String,
    email: String,
) -> User {
    User { name, email }
}

fn private_fn() {}
