use de_time::UnixMillis;

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum VisualStatus {
    Visible,
    Blank,
    Unknown,
}

impl VisualStatus {
    pub const ALL: [VisualStatus; 3] = [
        VisualStatus::Visible,
        VisualStatus::Blank,
        VisualStatus::Unknown,
    ];

    pub fn wire_name(&self) -> &'static str {
        match self {
            VisualStatus::Visible => "visible",
            VisualStatus::Blank => "blank",
            VisualStatus::Unknown => "unknown",
        }
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum CaptureClassification {
    InsufficientMappedOnly,
    ContentCommitted,
    FramePresented,
    CaptureVisible,
    BlankCaptureFailure,
    NotVisible,
}

impl CaptureClassification {
    pub const ALL: [CaptureClassification; 6] = [
        CaptureClassification::InsufficientMappedOnly,
        CaptureClassification::ContentCommitted,
        CaptureClassification::FramePresented,
        CaptureClassification::CaptureVisible,
        CaptureClassification::BlankCaptureFailure,
        CaptureClassification::NotVisible,
    ];

    pub fn wire_name(&self) -> &'static str {
        match self {
            CaptureClassification::InsufficientMappedOnly => "insufficient_mapped_only",
            CaptureClassification::ContentCommitted => "content_committed",
            CaptureClassification::FramePresented => "frame_presented",
            CaptureClassification::CaptureVisible => "capture_visible",
            CaptureClassification::BlankCaptureFailure => "blank_capture_failure",
            CaptureClassification::NotVisible => "not_visible",
        }
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct EvidencePacket {
    pub scenario: String,
    pub captured_at: UnixMillis,
    pub visual_status: VisualStatus,
    pub capture_classification: CaptureClassification,
}

#[cfg(test)]
mod tests {
    use super::{CaptureClassification, VisualStatus};

    #[test]
    fn visual_status_wire_names_are_stable() {
        let names: Vec<&str> = VisualStatus::ALL
            .iter()
            .map(VisualStatus::wire_name)
            .collect();
        assert_eq!(names, vec!["visible", "blank", "unknown"]);
    }

    #[test]
    fn capture_classification_wire_names_are_stable() {
        let names: Vec<&str> = CaptureClassification::ALL
            .iter()
            .map(CaptureClassification::wire_name)
            .collect();
        assert_eq!(
            names,
            vec![
                "insufficient_mapped_only",
                "content_committed",
                "frame_presented",
                "capture_visible",
                "blank_capture_failure",
                "not_visible",
            ]
        );
    }
}
