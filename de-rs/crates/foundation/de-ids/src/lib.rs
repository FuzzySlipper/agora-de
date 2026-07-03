#[derive(Clone, Debug, Eq, Hash, PartialEq)]
pub struct SurfaceId(String);

impl SurfaceId {
    pub fn new(value: impl Into<String>) -> Self {
        Self(value.into())
    }

    pub fn as_str(&self) -> &str {
        &self.0
    }
}

#[derive(Clone, Debug, Eq, Hash, PartialEq)]
pub struct LaunchId(String);

impl LaunchId {
    pub fn new(value: impl Into<String>) -> Self {
        Self(value.into())
    }
}

