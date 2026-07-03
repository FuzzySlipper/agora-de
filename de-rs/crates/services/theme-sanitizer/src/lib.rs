use protocol_shell::ThemeToken;

pub fn accepts_token(token: &ThemeToken) -> bool {
    token.is_agora_token()
}

