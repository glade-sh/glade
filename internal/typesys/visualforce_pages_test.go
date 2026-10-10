package typesys

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
)

func TestVisualforcePageInventoryRetainsMetadataAliases(t *testing.T) {
	p := project.Project{
		Namespace:            "Local",
		PackageDirectories:   []project.PackageDirectory{{Path: "force-app"}},
		VisualforcePageFiles: []string{"pages/Landing.page", "pages/Landing.page-meta.xml"},
		ManagedPackageDependencies: []project.ManagedPackageDependency{{
			Namespace: "Owned", Status: "loaded",
			Project: &project.Project{VisualforcePageFiles: []string{"pages/Detail.page-meta.xml"}},
		}},
	}
	want := []string{"detail", "landing", "local__landing", "owned__detail"}
	if got := projectVisualforcePageNames(p); !reflect.DeepEqual(got, want) {
		t.Fatalf("page aliases = %v, want %v", got, want)
	}
	if !projectVisualforcePagesKnown(p) {
		t.Fatal("discovered source projects must have a known page inventory")
	}
	p.ManagedPackageDependencies[0].Project = nil
	p.ManagedPackageDependencies[0].ArtifactPath = "owned-artifact.json"
	if projectVisualforcePagesKnown(p) {
		t.Fatal("artifact without page metadata cannot prove an absent page")
	}
}

func TestVisualforcePageInventoryBindsIndexIdentity(t *testing.T) {
	p := project.Project{
		Root:                 t.TempDir(),
		PackageDirectories:   []project.PackageDirectory{{Path: "force-app"}},
		VisualforcePageFiles: []string{"pages/Landing.page"},
	}
	index := Build(p, schema.Schema{})
	if !index.VisualforcePagesKnown || !reflect.DeepEqual(index.VisualforcePageNames, []string{"landing"}) {
		t.Fatalf("indexed page metadata = %v, known=%v", index.VisualforcePageNames, index.VisualforcePagesKnown)
	}
	raw, err := json.Marshal(index)
	if err != nil {
		t.Fatal(err)
	}
	var restored Index
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.VisualforcePagesKnown != index.VisualforcePagesKnown || !reflect.DeepEqual(restored.VisualforcePageNames, index.VisualforcePageNames) {
		t.Fatal("serialized index lost its page inventory")
	}
	if !MatchesProjectIdentity(index, p) {
		t.Fatal("unchanged page inventory changed project identity")
	}
	p.VisualforcePageFiles = append(p.VisualforcePageFiles, "pages/Following.page")
	if MatchesProjectIdentity(index, p) {
		t.Fatal("an added page must invalidate the incremental project identity")
	}
	if !reflect.DeepEqual(index.VisualforcePageNames, []string{"landing"}) {
		t.Fatal("later project discovery mutated the published page inventory")
	}
}

func TestVisualforcePageInventorySurvivesIncrementalApexEdit(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	path := filepath.Join(root, "force-app/main/default/classes/PageOwner.cls")
	writeFile(t, path, "public class PageOwner { public static Integer value() { return 1; } }")
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/OwnedLanding.page"), "<apex:page>Owned</apex:page>")
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	previous := Build(p, schema.Schema{})
	if previous.HasErrors() || !previous.VisualforcePagesKnown || len(previous.VisualforcePageNames) != 1 {
		t.Fatalf("initial page inventory was not captured: %#v", previous)
	}
	writeFile(t, path, "public class PageOwner { public static Integer value() { return 2; } }")
	updated, fast := updateApexFilesIncremental(previous, []string{path}, nil)
	if !fast {
		t.Fatal("ordinary Apex edit did not use the incremental path")
	}
	if updated.VisualforcePagesKnown != previous.VisualforcePagesKnown || !reflect.DeepEqual(updated.VisualforcePageNames, previous.VisualforcePageNames) {
		t.Fatalf("Apex edit lost the page inventory: names=%v known=%v", updated.VisualforcePageNames, updated.VisualforcePagesKnown)
	}
}
