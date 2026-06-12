package lwcbrowser

import (
	"testing"

	"github.com/glade-sh/glade/internal/aura"
)

func TestApplyAuraLWCPassthroughAliases(t *testing.T) {
	manifest := Manifest{
		Modules: map[string]ModuleEntry{
			"verifiable:setup": {
				URL: "/lightning/modules/verifiable/setup/setup.js",
				Tag: "verifiable-setup",
			},
		},
	}
	ApplyAuraLWCPassthroughAliases(manifest, []aura.LWCPassthrough{{
		AuraName: "setupAssistant",
		Target:   "verifiable:setup",
	}}, "verifiable")

	entry, ok := manifest.Modules["verifiable:setupassistant"]
	if !ok {
		t.Fatal("missing setupAssistant alias")
	}
	if entry.URL != "/lightning/modules/verifiable/setup/setup.js" {
		t.Fatalf("alias URL = %q", entry.URL)
	}
}

func TestLocalLWCImportMapNamespacedPackage(t *testing.T) {
	manifest := Manifest{
		Modules: map[string]ModuleEntry{
			"verifiable:landing": {
				URL: "/lightning/modules/verifiable/landing/landing.js",
				Tag: "verifiable-landing",
			},
			"verifiable:wizard": {
				URL: "/lightning/modules/verifiable/wizard/wizard.js",
				Tag: "verifiable-wizard",
			},
		},
	}
	imports := LocalLWCImportMap("verifiable", manifest)
	if imports["c/landing"] != "/lightning/modules/verifiable/landing/landing.js" {
		t.Fatalf("c/landing = %q", imports["c/landing"])
	}
	if imports["verifiable/landing"] != "/lightning/modules/verifiable/landing/landing.js" {
		t.Fatalf("verifiable/landing = %q", imports["verifiable/landing"])
	}
	if imports["c/wizard"] != "/lightning/modules/verifiable/wizard/wizard.js" {
		t.Fatalf("c/wizard = %q", imports["c/wizard"])
	}
}

func TestLocalLWCImportMapDefaultNamespace(t *testing.T) {
	manifest := Manifest{
		Modules: map[string]ModuleEntry{
			"c:counter": {
				URL: "/lightning/modules/c/counter/counter.js",
				Tag: "c-counter",
			},
		},
	}
	imports := LocalLWCImportMap("c", manifest)
	if imports["c/counter"] != "/lightning/modules/c/counter/counter.js" {
		t.Fatalf("c/counter = %q", imports["c/counter"])
	}
	if _, ok := imports["verifiable/counter"]; ok {
		t.Fatalf("unexpected verifiable alias: %#v", imports)
	}
}
