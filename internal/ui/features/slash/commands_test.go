package slash

import "testing"

func TestNormalizeCommandResolvesAliases(t *testing.T) {
	tests := map[string]string{
		"/models":   "/model",
		"/ctx":      "/memory",
		"/context":  "/memory",
		"/sessions": "/session",
		"/worktree": "/workspace",
		"/settings": "/config",
	}

	for input, want := range tests {
		if got := NormalizeCommand(input); got != want {
			t.Fatalf("NormalizeCommand(%q)=%q, want %q", input, got, want)
		}
	}
}

func TestGetSuggestionsMatchesAliasesWithoutDuplicates(t *testing.T) {
	items := GetSuggestions("/set")
	if len(items) != 1 {
		t.Fatalf("expected 1 suggestion, got %d", len(items))
	}
	if items[0].Name != "/config" {
		t.Fatalf("expected /config suggestion, got %q", items[0].Name)
	}
}

func TestGroupedVisibleCommandsPreservesExpectedGroups(t *testing.T) {
	groups := GroupedVisibleCommands("zh")
	if len(groups) < 4 {
		t.Fatalf("expected at least 4 groups, got %d", len(groups))
	}
	if groups[0].Group != GroupGeneral {
		t.Fatalf("expected first group to be general, got %q", groups[0].Group)
	}
	if groups[1].Group != GroupProject {
		t.Fatalf("expected second group to be project, got %q", groups[1].Group)
	}
}

func TestVisibleCommandsIncludesReloadPlugins(t *testing.T) {
	if NormalizeCommand("/reload-plugins") != "/reload-plugins" {
		t.Fatalf("expected /reload-plugins to be a visible slash command")
	}
}
