#[derive(Clone, Debug, Eq, PartialEq)]
pub enum DeErrorKind {
    InvalidInput,
    BoundaryViolation,
    Unavailable,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct DeError {
    pub kind: DeErrorKind,
    pub message: String,
}

impl DeError {
    pub fn new(kind: DeErrorKind, message: impl Into<String>) -> Self {
        Self {
            kind,
            message: message.into(),
        }
    }
}
