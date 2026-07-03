use de_ids::SurfaceId;

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum SurfaceEventKind {
    Mapped,
    Unmapped,
    Focused,
    InputDenied,
}

impl SurfaceEventKind {
    pub const ALL: [SurfaceEventKind; 4] = [
        SurfaceEventKind::Mapped,
        SurfaceEventKind::Unmapped,
        SurfaceEventKind::Focused,
        SurfaceEventKind::InputDenied,
    ];

    pub fn wire_name(&self) -> &'static str {
        match self {
            SurfaceEventKind::Mapped => "mapped",
            SurfaceEventKind::Unmapped => "unmapped",
            SurfaceEventKind::Focused => "focused",
            SurfaceEventKind::InputDenied => "input_denied",
        }
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SurfaceEvent {
    pub surface_id: SurfaceId,
    pub kind: SurfaceEventKind,
    pub owner_uid: u32,
}

#[cfg(test)]
mod tests {
    use super::SurfaceEventKind;

    #[test]
    fn surface_event_wire_names_are_stable() {
        let names: Vec<&str> = SurfaceEventKind::ALL
            .iter()
            .map(SurfaceEventKind::wire_name)
            .collect();
        assert_eq!(names, vec!["mapped", "unmapped", "focused", "input_denied"]);
    }
}
