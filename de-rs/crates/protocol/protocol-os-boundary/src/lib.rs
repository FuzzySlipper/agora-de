use de_time::UnixMillis;

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AdminEscalationSummary {
    pub id: String,
    pub requested_at: UnixMillis,
    pub actor_uid: u32,
}

