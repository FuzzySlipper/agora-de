use de_ids::SurfaceId;

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SurfaceRecord {
    pub id: SurfaceId,
    pub owner_uid: u32,
}

