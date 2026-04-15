/// Greet by printing a name.
pub fn greet(name: []const u8) void {
    _ = name;
}

// Add two i32 values.
pub fn add(
    a: i32,
    b: i32,
) i32 {
    return a + b;
}

fn helper() void {} // private helper
