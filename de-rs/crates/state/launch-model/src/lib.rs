use de_ids::LaunchId;

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct LaunchRecord {
    pub id: LaunchId,
    pub requester_uid: u32,
}

