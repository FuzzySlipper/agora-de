package capture

import "time"

type VisualInspectionStatus string

const (
	VisualInspectionUnknown VisualInspectionStatus = "unknown"
	VisualInspectionVisible VisualInspectionStatus = "visible"
	VisualInspectionBlank   VisualInspectionStatus = "blank"
)

type Evidence struct {
	Mapped               bool
	FrameCount           int
	LastPresentTimestamp time.Time
	CaptureCount         int
	LastCaptureTimestamp time.Time
	CapturedAt           time.Time
	VisualInspection     VisualInspectionStatus
}

type Classification string

const (
	ClassificationInsufficientMappedOnly Classification = "insufficient_mapped_only"
	ClassificationFramePresented         Classification = "frame_presented"
	ClassificationCaptureVisible         Classification = "capture_visible"
	ClassificationBlankCaptureFailure    Classification = "blank_capture_failure"
	ClassificationNotVisible             Classification = "not_visible"
)

func Classify(evidence Evidence, now time.Time) Classification {
	if !evidence.Mapped {
		return ClassificationNotVisible
	}
	if evidence.FrameCount > 0 && !evidence.LastPresentTimestamp.IsZero() {
		return ClassificationFramePresented
	}
	if evidence.VisualInspection == VisualInspectionBlank {
		return ClassificationBlankCaptureFailure
	}
	if evidence.VisualInspection == VisualInspectionVisible &&
		evidence.CaptureCount > 0 &&
		!evidence.LastCaptureTimestamp.IsZero() &&
		!evidence.CapturedAt.IsZero() &&
		!evidence.CapturedAt.After(now) {
		return ClassificationCaptureVisible
	}
	return ClassificationInsufficientMappedOnly
}

