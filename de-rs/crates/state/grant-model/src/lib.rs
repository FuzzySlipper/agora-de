#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ViewportGrant {
    pub grantee_uid: u32,
    pub surface_owner_uid: u32,
}
