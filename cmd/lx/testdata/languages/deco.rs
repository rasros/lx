#[derive(Debug, Clone)]
pub struct Point {
    pub x: i32,
}

#[inline]
pub fn origin() -> Point {
    Point { x: 0 }
}
