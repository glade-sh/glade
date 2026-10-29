package sema

import (
	"os"
	"path/filepath"
	"testing"
)

// Indexed units belong to the model's immutable build. A file absent from that
// build has no captured marker identity: each fallback must observe its supplied
// root and the current filesystem, including negative-to-positive transitions.
func TestSemaSourceUnitFallbackObservesRootAndMarkerEdits(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "nested")
	file := filepath.Join(nested, "classes", "Probe.cls")
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		t.Fatal(err)
	}
	outer := filepath.Join(root, "sfdx-project.json")
	inner := filepath.Join(nested, "sfdx-project.json")
	writeSemaFile(t, outer, `{}`)
	holder := &semaTypeMemberModel{}
	check := func(sourceRoot string) {
		t.Helper()
		want := semaSourceUnitKey(file, sourceRoot)
		if got := holder.sourceUnitForFile(file, sourceRoot); got != want {
			t.Errorf("fallback root=%q got=%q want current walk=%q", sourceRoot, got, want)
		}
	}
	check(root)
	check(nested) // Same file, a different walk boundary.
	writeSemaFile(t, inner, `{}`)
	check(root) // A nearer marker appears after a negative lookup.
	if err := os.Remove(inner); err != nil {
		t.Fatal(err)
	}
	check(root)
	if err := os.Remove(outer); err != nil {
		t.Fatal(err)
	}
	check(root) // A formerly positive marker disappears.
	writeSemaFile(t, inner, `{}`)
	check(nested)
	// A second model never inherits another model's fallback answers.
	other := &semaTypeMemberModel{}
	if got := other.sourceUnitForFile(file, nested); got != semaSourceUnitKey(file, nested) {
		t.Fatal(got)
	}
}

func TestSemaKnownTypeHoldersKeepProjectNamesPrivate(t *testing.T) {
	// The process-wide platform set is immutable. Project additions and changes
	// belong only to their checker, independent of API/org/test lifetime.
	platformName := ""
	for name := range semaPlatformKnownTypes() {
		platformName = name
		break
	}
	if platformName == "" {
		t.Fatal("empty platform set")
	}
	for _, name := range []string{"ownedone", "ownedtwo", "ownedthree"} {
		left := querySemanticsChecker{knownTypes: map[string]bool{name: true}}
		right := querySemanticsChecker{knownTypes: map[string]bool{}}
		if !left.isKnownType(name) || right.isKnownType(name) || semaPlatformKnownTypes()[name] {
			t.Fatalf("project name leaked between checker holders: %s", name)
		}
		delete(left.knownTypes, name)
		left.knownTypes[platformName] = false
		if left.isKnownType(name) || !right.isKnownType(platformName) {
			t.Fatal("holder mutation changed the platform set")
		}
	}
}
