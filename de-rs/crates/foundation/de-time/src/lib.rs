#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct UnixMillis(pub u64);

impl UnixMillis {
    pub const ZERO: Self = Self(0);
}

