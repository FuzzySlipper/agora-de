use de_time::UnixMillis;

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AdminEscalationSummary {
    pub id: String,
    pub requested_at: UnixMillis,
    pub actor_uid: u32,
    pub summary: String,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum AgentState {
    Unknown,
    Ready,
    Busy,
    Offline,
}

impl AgentState {
    pub const ALL: [AgentState; 4] = [
        AgentState::Unknown,
        AgentState::Ready,
        AgentState::Busy,
        AgentState::Offline,
    ];

    pub fn wire_name(self) -> &'static str {
        match self {
            AgentState::Unknown => "unknown",
            AgentState::Ready => "ready",
            AgentState::Busy => "busy",
            AgentState::Offline => "offline",
        }
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AgentInfo {
    pub uid: u32,
    pub display_name: String,
    pub state: AgentState,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AuditEvent {
    pub id: String,
    pub actor_uid: u32,
    pub action: String,
    pub subject: String,
    pub created_at: UnixMillis,
}
