package startupcache

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/typesys"
)

// The cached org shape is the effective project's. Adding or removing Person
// Account evidence in a declared dependency changes the manifest features, so
// a runtime built for the earlier shape is not reused.
func TestBuildManifestFeaturesFollowDependencyOrgShapeEvidence(t *testing.T) {
	workspace := t.TempDir()
	consumer := filepath.Join(workspace, "consumer")
	base := filepath.Join(workspace, "base")
	writeStartupCacheTestFile(t, filepath.Join(consumer, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true,"dependencies":[{"package":"PersonBase"}]}]}`)
	writeStartupCacheTestFile(t, filepath.Join(consumer, "glade.yml"), "project:\n  dependencyPaths: [\"../base\"]\n")
	writeStartupCacheTestFile(t, filepath.Join(consumer, "force-app/main/default/classes/Consumer.cls"), "public class Consumer {}\n")
	writeStartupCacheTestFile(t, filepath.Join(base, "sfdx-project.json"), `{"packageDirectories":[{"path":"base-app","default":true,"package":"PersonBase"}]}`)
	writeStartupCacheTestFile(t, filepath.Join(base, "base-app/main/default/classes/Base.cls"), "public class Base {}\n")
	features := func() []string {
		t.Helper()
		loaded, err := project.Load(consumer)
		if err != nil {
			t.Fatal(err)
		}
		return BuildManifest(consumer, loaded, typesys.Index{}).Features
	}
	if got := features(); slices.Contains(got, "PersonAccounts") {
		t.Fatalf("features without evidence = %v", got)
	}
	evidence := filepath.Join(base, "base-app/main/default/objects/PersonAccount/recordTypes/PersonAccount.recordType-meta.xml")
	writeStartupCacheTestFile(t, evidence, `<RecordType xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>PersonAccount</fullName><active>true</active><label>Person Account</label></RecordType>`)
	if got := features(); !slices.Contains(got, "PersonAccounts") {
		t.Fatalf("dependency record type evidence missing from manifest features: %v", got)
	}
	if err := os.Remove(evidence); err != nil {
		t.Fatal(err)
	}
	writeStartupCacheTestFile(t, filepath.Join(base, "config/project-scratch-def.json"), `{"features":["PersonAccounts","MultiCurrency"]}`)
	got := features()
	if !slices.Contains(got, "PersonAccounts") || !slices.Contains(got, "MultiCurrency") {
		t.Fatalf("dependency scratch features missing from manifest features: %v", got)
	}
}
