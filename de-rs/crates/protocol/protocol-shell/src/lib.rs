#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ThemeToken {
    pub name: String,
    pub value: String,
}

impl ThemeToken {
    pub fn is_agora_token(&self) -> bool {
        self.name.starts_with("--agora-")
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct CatalogApp {
    pub id: String,
    pub name: String,
    pub icon: String,
}
