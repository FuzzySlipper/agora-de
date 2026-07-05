package catalog

import (
	"testing"

	"agora-de.local/go/internal/appcatalog"
)

func TestVisibleAppViews(t *testing.T) {
	source := appcatalog.NewCatalog()
	source.Add(appcatalog.Entry{ID: "browser", Name: "Browser", Exec: "browser", Icon: "browser"})
	source.Add(appcatalog.Entry{ID: "hidden", Name: "Hidden", Exec: "hidden", NoDisplay: true})

	views := VisibleAppViews(source)
	if len(views) != 1 {
		t.Fatalf("views = %d, want 1", len(views))
	}
	if views[0].ID != "browser" {
		t.Fatalf("view id = %q, want browser", views[0].ID)
	}
	if views[0].Name != "Browser" {
		t.Fatalf("view name = %q, want Browser", views[0].Name)
	}
	if views[0].DisabledReason != "" {
		t.Fatalf("disabled reason = %q, want empty", views[0].DisabledReason)
	}
}

func TestVisibleAppViewsIncludesDisabledReasonForUnsupportedEntries(t *testing.T) {
	source := appcatalog.NewCatalog()
	source.Add(appcatalog.Entry{ID: "broken", Name: "Broken", Exec: "broken %Z", Icon: "broken"})

	views := VisibleAppViews(source)
	if len(views) != 1 {
		t.Fatalf("views = %d, want 1", len(views))
	}
	if views[0].Launchable {
		t.Fatalf("broken app should not be launchable: %+v", views[0])
	}
	if views[0].DisabledReason == "" {
		t.Fatalf("disabled reason should be present: %+v", views[0])
	}
}
