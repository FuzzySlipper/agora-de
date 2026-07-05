package catalog

import (
	"testing"

	"agora-de.local/go/internal/appcatalog"
)

func TestVisibleAppViews(t *testing.T) {
	source := appcatalog.NewCatalog()
	source.Add(appcatalog.Entry{ID: "browser", Name: "Browser", Exec: "browser", Icon: "browser", Categories: []string{"Network", "WebBrowser"}})
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
	if views[0].IconKind != "theme" || views[0].IconRef != "browser" {
		t.Fatalf("icon projection = kind %q ref %q, want theme/browser", views[0].IconKind, views[0].IconRef)
	}
	if views[0].IconLabel != "B" {
		t.Fatalf("icon label = %q, want B", views[0].IconLabel)
	}
	if views[0].Category != "Internet" {
		t.Fatalf("category = %q, want Internet", views[0].Category)
	}
	if views[0].DisabledReason != "" {
		t.Fatalf("disabled reason = %q, want empty", views[0].DisabledReason)
	}
}

func TestIconProjectionHandlesAbsoluteAndFallbackIcons(t *testing.T) {
	if IconKind("/usr/share/pixmaps/app.svg") != "path" {
		t.Fatal("absolute icon should project as path")
	}
	if IconKind("") != "fallback" || IconRef("") != "application-x-executable" {
		t.Fatalf("empty icon projection = %q/%q, want fallback/application-x-executable", IconKind(""), IconRef(""))
	}
}

func TestCategoryGroupFallsBackToOther(t *testing.T) {
	if got := CategoryGroup(nil); got != "Other" {
		t.Fatalf("category group = %q, want Other", got)
	}
	if got := CategoryGroup([]string{"Utility"}); got != "Utilities" {
		t.Fatalf("category group = %q, want Utilities", got)
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
