#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SessionToken(String);

impl SessionToken {
    pub fn new(value: impl Into<String>) -> Self {
        Self(value.into())
    }
}
