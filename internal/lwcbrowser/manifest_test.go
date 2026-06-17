package lwcbrowser

import (
	"testing"

	"github.com/glade-sh/glade/internal/aura"
)

func TestApplyAuraLWCPassthroughAliases(t *testing.T) {
	manifest := Manifest{
		Modules: map[string]ModuleEntry{
			"samplepkg:setup": {
				URL: "/lightning/modules/samplepkg/setup/setup.js",
				Tag: "samplepkg-setup",
			},
		},
	}
	ApplyAuraLWCPassthroughAliases(manifest, []aura.LWCPassthrough{{
		AuraName: "setupAssistant",
		Target:   "samplepkg:setup",
	}}, "samplepkg")

	entry, ok := manifest.Modules["samplepkg:setupassistant"]
	if !ok {
		t.Fatal("missing setupAssistant alias")
	}
	if entry.URL != "/lightning/modules/samplepkg/setup/setup.js" {
		t.Fatalf("alias URL = %q", entry.URL)
	}
}

func TestLocalLWCImportMapNamespacedPackage(t *testing.T) {
	manifest := Manifest{
		Modules: map[string]ModuleEntry{
			"samplepkg:landing": {
				URL: "/lightning/modules/samplepkg/landing/landing.js",
				Tag: "samplepkg-landing",
			},
			"samplepkg:wizard": {
				URL: "/lightning/modules/samplepkg/wizard/wizard.js",
				Tag: "samplepkg-wizard",
			},
		},
	}
	imports := LocalLWCImportMap("samplepkg", manifest)
	if imports["c/landing"] != "/lightning/modules/samplepkg/landing/landing.js" {
		t.Fatalf("c/landing = %q", imports["c/landing"])
	}
	if imports["samplepkg/landing"] != "/lightning/modules/samplepkg/landing/landing.js" {
		t.Fatalf("samplepkg/landing = %q", imports["samplepkg/landing"])
	}
	if imports["c/wizard"] != "/lightning/modules/samplepkg/wizard/wizard.js" {
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
	if _, ok := imports["samplepkg/counter"]; ok {
		t.Fatalf("unexpected samplepkg alias: %#v", imports)
	}
}
