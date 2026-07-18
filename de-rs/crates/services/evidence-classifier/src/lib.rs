use protocol_evidence::{EvidencePacket, VisualStatus};

pub fn is_user_visible(packet: &EvidencePacket) -> bool {
    packet.visual_status == VisualStatus::Visible
}
