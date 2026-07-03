use de_ids::SurfaceId;

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum SurfaceEventKind {
    Mapped,
    Unmapped,
    Focused,
    InputDenied,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SurfaceEvent {
    pub surface_id: SurfaceId,
    pub kind: SurfaceEventKind,
    pub owner_uid: u32,
}

