package ai

import (
	"testing"

	"github.com/dreamSailing/eos/pkg/coreapi"
)

func TestApplyCoreModelCatalogReplacesProviderAndPresetLookups(t *testing.T) {
	t.Cleanup(func() {
		globalRegistry = NewProviderRegistry()
		globalCatalog = NewModelCatalog()
		globalResolver = NewResolver()
	})

	ApplyCoreModelCatalog(coreapi.ModelCatalogState{
		Providers: []coreapi.ModelProviderOption{{
			ID:             "example",
			Name:           "Example",
			DefaultAPIBase: "https://example.invalid/v1",
			DefaultModels:  []string{"example-vision"},
		}},
		Presets: []coreapi.ModelPresetOption{{
			ID:             "example-vision",
			Name:           "Example Vision",
			ProviderID:     "example",
			ModelName:      "example-vision",
			APIType:        "standard",
			ContextWindow:  12345,
			SupportsVision: true,
			SupportsTools:  true,
		}},
		AllowCustomProvider: true,
		AllowCustomModel:    true,
	})

	providers := GetAllProviders()
	if len(providers) != 1 || providers[0].ID != "example" {
		t.Fatalf("providers=%+v, want runtime catalog provider only", providers)
	}
	if got := GetProviderByID("example"); got == nil || got.DefaultAPIBase != "https://example.invalid/v1" {
		t.Fatalf("GetProviderByID(example)=%+v", got)
	}
	entry := GetModelEntry("example-vision")
	if entry == nil {
		t.Fatal("GetModelEntry(example-vision)=nil")
	}
	if entry.Provider != ProviderType("example") || entry.ContextWindow != 12345 {
		t.Fatalf("entry=%+v", entry)
	}
	if !SupportsVisionFromCatalog("example-vision") {
		t.Fatal("SupportsVisionFromCatalog(example-vision)=false, want true")
	}
	if !SupportsToolsFromCatalog("example-vision") {
		t.Fatal("SupportsToolsFromCatalog(example-vision)=false, want true")
	}
}

func TestApplyCoreModelCatalogEmptySnapshotStaysEmpty(t *testing.T) {
	t.Cleanup(func() {
		globalRegistry = NewProviderRegistry()
		globalCatalog = NewModelCatalog()
		globalResolver = NewResolver()
	})

	ApplyCoreModelCatalog(coreapi.ModelCatalogState{})

	if providers := GetAllProviders(); len(providers) != 0 {
		t.Fatalf("providers=%+v, want empty catalog", providers)
	}
	if models := GetAllModels(); len(models) != 0 {
		t.Fatalf("models=%+v, want empty catalog", models)
	}
	if SupportsVisionFromCatalog("example-vision") {
		t.Fatal("SupportsVisionFromCatalog(example-vision)=true, want false for empty snapshot")
	}
	if SupportsToolsFromCatalog("example-vision") {
		t.Fatal("SupportsToolsFromCatalog(example-vision)=true, want false for empty snapshot")
	}
	if AllowCustomProviderFromCatalog() {
		t.Fatal("AllowCustomProviderFromCatalog()=true, want false for empty snapshot")
	}
	if AllowCustomModelFromCatalog() {
		t.Fatal("AllowCustomModelFromCatalog()=true, want false for empty snapshot")
	}
}
