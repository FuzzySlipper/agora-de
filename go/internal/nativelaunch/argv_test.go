package nativelaunch

import (
	"errors"
	"reflect"
	"testing"

	"agora-de.local/go/internal/appcatalog"
)

func TestBuildArgvSubstitutesSupportedFieldCodes(t *testing.T) {
	entry := appcatalog.Entry{
		Type: "Application",
		Name: "Sample App",
		Exec: `sample --name "%c" --desktop %k percent %%`,
	}

	got, err := BuildArgv(entry, "/usr/share/applications/sample.desktop")
	if err != nil {
		t.Fatalf("BuildArgv error = %v", err)
	}
	want := []string{
		"sample",
		"--name",
		"Sample App",
		"--desktop",
		"/usr/share/applications/sample.desktop",
		"percent",
		"%",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("argv = %#v, want %#v", got, want)
	}
}

func TestBuildArgvOmitsSelectionFieldCodesForPlainLaunch(t *testing.T) {
	got, err := BuildArgv(appcatalog.Entry{Name: "Browser", Exec: "browser %u %U %f %F %i --new-window"}, "")
	if err != nil {
		t.Fatalf("BuildArgv error = %v", err)
	}
	want := []string{"browser", "--new-window"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("argv = %#v, want %#v", got, want)
	}
}

func TestBuildArgvAddsWaylandHintForChromiumFamily(t *testing.T) {
	for _, execValue := range []string{"brave %U", "/usr/bin/chromium %U"} {
		got, err := BuildArgv(appcatalog.Entry{Name: "Browser", Exec: execValue}, "")
		if err != nil {
			t.Fatalf("BuildArgv(%q) error = %v", execValue, err)
		}
		if got[len(got)-1] != "--ozone-platform=wayland" {
			t.Fatalf("BuildArgv(%q) = %#v, want ozone wayland hint", execValue, got)
		}
	}
}

func TestBuildArgvDoesNotDuplicateOzoneHint(t *testing.T) {
	got, err := BuildArgv(appcatalog.Entry{Name: "Browser", Exec: "chromium --ozone-platform=wayland %U"}, "")
	if err != nil {
		t.Fatalf("BuildArgv error = %v", err)
	}
	want := []string{"chromium", "--ozone-platform=wayland"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("argv = %#v, want %#v", got, want)
	}
}

func TestBuildArgvRejectsUnknownFieldCodes(t *testing.T) {
	_, err := BuildArgv(appcatalog.Entry{Name: "Browser", Exec: "browser %Z"}, "")
	if !errors.Is(err, ErrUnsupportedFieldCode) {
		t.Fatalf("BuildArgv error = %v, want ErrUnsupportedFieldCode", err)
	}
}

func TestCanPrepareMatchesStructuredArgvSupport(t *testing.T) {
	if !CanPrepare(appcatalog.Entry{
		ID:   "sample.desktop",
		Type: "Application",
		Name: "Sample",
		Exec: "sample --name %c",
	}) {
		t.Fatal("CanPrepare rejected supported argv entry")
	}
	if !CanPrepare(appcatalog.Entry{
		ID:   "browser.desktop",
		Type: "Application",
		Name: "Browser",
		Exec: "browser %U",
	}) {
		t.Fatal("CanPrepare rejected URL field code omitted for a plain launch")
	}
	if CanPrepare(appcatalog.Entry{
		ID:        "hidden.desktop",
		Type:      "Application",
		Name:      "Hidden",
		Exec:      "hidden",
		NoDisplay: true,
	}) {
		t.Fatal("CanPrepare accepted hidden entry")
	}
}

func TestBuildArgvKeepsShellSyntaxLiteral(t *testing.T) {
	entry := appcatalog.Entry{
		Name: "Literal",
		Exec: `literal "$HOME" "two words" "semi;colon"`,
	}

	got, err := BuildArgv(entry, "")
	if err != nil {
		t.Fatalf("BuildArgv error = %v", err)
	}
	want := []string{"literal", "$HOME", "two words", "semi;colon"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("argv = %#v, want %#v", got, want)
	}
}

func TestBuildArgvRejectsUnterminatedQuote(t *testing.T) {
	_, err := BuildArgv(appcatalog.Entry{Name: "Broken", Exec: `broken "unterminated`}, "")
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("BuildArgv error = %v, want ErrInvalidRequest", err)
	}
}
