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

func TestBuildArgvRejectsUnsupportedFieldCodes(t *testing.T) {
	_, err := BuildArgv(appcatalog.Entry{Name: "Browser", Exec: "browser %u"}, "")
	if !errors.Is(err, ErrUnsupportedFieldCode) {
		t.Fatalf("BuildArgv error = %v, want ErrUnsupportedFieldCode", err)
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
