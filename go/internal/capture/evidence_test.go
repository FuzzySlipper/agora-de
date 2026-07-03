package capture

import (
	"testing"
	"time"
)

func TestClassifyMappedOnlyIsInsufficient(t *testing.T) {
	got := Classify(Evidence{Mapped: true}, time.Now())
	if got != ClassificationInsufficientMappedOnly {
		t.Fatalf("classification = %q, want %q", got, ClassificationInsufficientMappedOnly)
	}
}

func TestClassifyFramePresented(t *testing.T) {
	now := time.Now()
	got := Classify(Evidence{
		Mapped:               true,
		FrameCount:           1,
		LastPresentTimestamp: now.Add(-time.Second),
	}, now)
	if got != ClassificationFramePresented {
		t.Fatalf("classification = %q, want %q", got, ClassificationFramePresented)
	}
}

func TestClassifyVisibleCaptureFallback(t *testing.T) {
	now := time.Now()
	got := Classify(Evidence{
		Mapped:               true,
		CaptureCount:         1,
		LastCaptureTimestamp: now.Add(-time.Second),
		CapturedAt:           now.Add(-time.Second),
		VisualInspection:     VisualInspectionVisible,
	}, now)
	if got != ClassificationCaptureVisible {
		t.Fatalf("classification = %q, want %q", got, ClassificationCaptureVisible)
	}
}

func TestClassifyBlankCaptureFailureEvenWhenMapped(t *testing.T) {
	got := Classify(Evidence{
		Mapped:           true,
		VisualInspection: VisualInspectionBlank,
	}, time.Now())
	if got != ClassificationBlankCaptureFailure {
		t.Fatalf("classification = %q, want %q", got, ClassificationBlankCaptureFailure)
	}
}

func TestClassifyUnmappedNotVisible(t *testing.T) {
	got := Classify(Evidence{Mapped: false}, time.Now())
	if got != ClassificationNotVisible {
		t.Fatalf("classification = %q, want %q", got, ClassificationNotVisible)
	}
}

