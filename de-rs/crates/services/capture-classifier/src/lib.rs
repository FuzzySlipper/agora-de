use protocol_evidence::VisualStatus;

pub fn classify_blank(nonzero_pixels: bool) -> VisualStatus {
    if nonzero_pixels {
        VisualStatus::Visible
    } else {
        VisualStatus::Blank
    }
}

