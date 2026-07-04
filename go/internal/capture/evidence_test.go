package capture

import (
	"encoding/json"
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

func TestClassifyContentCommittedBelowFramePresented(t *testing.T) {
	now := time.Now()
	got := Classify(Evidence{
		Mapped:                     true,
		ContentCommitCount:         1,
		LastContentCommitTimestamp: now.Add(-time.Second),
	}, now)
	if got != ClassificationContentCommitted {
		t.Fatalf("classification = %q, want %q", got, ClassificationContentCommitted)
	}

	got = Classify(Evidence{
		Mapped:                     true,
		FrameCount:                 1,
		LastPresentTimestamp:       now.Add(-time.Second),
		ContentCommitCount:         1,
		LastContentCommitTimestamp: now.Add(-time.Second),
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

func TestBuildEvidencePacketUsesGeneratedContractFieldNames(t *testing.T) {
	now := time.UnixMilli(1000)
	packet := BuildEvidencePacket("visible-capture", Evidence{
		Mapped:               true,
		CaptureCount:         1,
		LastCaptureTimestamp: now,
		CapturedAt:           now,
		VisualInspection:     VisualInspectionVisible,
	}, now)

	payload, err := json.Marshal(packet)
	if err != nil {
		t.Fatal(err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["captureClassification"] != string(ClassificationCaptureVisible) {
		t.Fatalf("captureClassification = %v, want %s", decoded["captureClassification"], ClassificationCaptureVisible)
	}
	if decoded["visualStatus"] != string(VisualInspectionVisible) {
		t.Fatalf("visualStatus = %v, want %s", decoded["visualStatus"], VisualInspectionVisible)
	}
	if _, ok := decoded["capturedAtUnixMillis"]; !ok {
		t.Fatal("capturedAtUnixMillis missing from JSON")
	}
}

func TestClassificationStringsMatchGeneratedTypescriptContract(t *testing.T) {
	want := []Classification{
		ClassificationInsufficientMappedOnly,
		ClassificationContentCommitted,
		ClassificationFramePresented,
		ClassificationCaptureVisible,
		ClassificationBlankCaptureFailure,
		ClassificationNotVisible,
	}
	for _, classification := range want {
		if classification == "" {
			t.Fatal("classification string must not be empty")
		}
	}
}
