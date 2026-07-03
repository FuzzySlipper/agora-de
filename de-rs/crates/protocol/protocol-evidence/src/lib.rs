use de_time::UnixMillis;

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum VisualStatus {
    Visible,
    Blank,
    Unknown,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum CaptureClassification {
    InsufficientMappedOnly,
    FramePresented,
    CaptureVisible,
    BlankCaptureFailure,
    NotVisible,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct EvidencePacket {
    pub scenario: String,
    pub captured_at: UnixMillis,
    pub visual_status: VisualStatus,
    pub capture_classification: CaptureClassification,
}
