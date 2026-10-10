package resource

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/namespaceremap"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func dataWeaveProject(t *testing.T, namespace, projectAPI, componentAPI, source string) project.Project {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"`+namespace+`","sourceApiVersion":"`+projectAPI+`"}`)
	path := filepath.Join(root, "force-app/main/default/dw/Owned.dwl")
	writeFile(t, path, source)
	if componentAPI != "" {
		writeFile(t, path+"-meta.xml", `<DataWeaveResource xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+componentAPI+`</apiVersion><isGlobal>false</isGlobal></DataWeaveResource>`)
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func TestDataWeaveProjectPreservesSourceAndComponentVersion(t *testing.T) {
	const source = "%dw 2.0\r\noutput application/json\r\n---\r\n  3 + 4  \r\n"
	p := dataWeaveProject(t, "owned", "67.0", "53.0", source)
	first, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.DataWeaveResources) != 1 {
		t.Fatalf("missing script: %+v", first.DataWeaveResources)
	}
	script := first.DataWeaveResources[0]
	if script.Name != "Owned" || script.Namespace != "owned" || script.Content != source || script.APIVersion != "53.0" || script.ContentPath != p.DataWeaveFiles[0] || script.MetadataPath != p.DataWeaveMetas[0] {
		t.Fatalf("script changed: %+v", script)
	}
	const changed = "%dw 2.0\noutput application/json\n---\n3 + 9\n"
	writeFile(t, p.DataWeaveFiles[0], changed)
	second, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	if second.DataWeaveResources[0].Content != changed || first.DataWeaveResources[0].Content != source {
		t.Fatal("same-name source update was stale or changed previous registry")
	}
	org := storage.OrgState{Objects: map[string]storage.ObjectState{}}
	if err := ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	if org.Metadata.DataWeaveResources[0].Content != changed {
		t.Fatal("ApplyProject lost actual source")
	}
}
func TestDataWeaveProjectVersionFallbackAndMissingBody(t *testing.T) {
	p := dataWeaveProject(t, "", "66.0", "", "")
	orphan := filepath.Join(p.Root, "force-app/main/default/dw/OnlyMetadata.dwl-meta.xml")
	writeFile(t, orphan, `<DataWeaveResource><apiVersion>64.0</apiVersion></DataWeaveResource>`)
	p.DataWeaveMetas = append(p.DataWeaveMetas, orphan)
	r, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]storage.DataWeaveResourceMetadata{}
	for _, s := range r.DataWeaveResources {
		byName[s.Name] = s
	}
	if byName["Owned"].APIVersion != "66.0" || byName["Owned"].ContentPath == "" || byName["Owned"].Content != "" {
		t.Fatalf("empty source/fallback lost: %+v", byName["Owned"])
	}
	if byName["OnlyMetadata"].ContentPath != "" || byName["OnlyMetadata"].MetadataPath != orphan || byName["OnlyMetadata"].APIVersion != "64.0" {
		t.Fatalf("orphan misrepresented: %+v", byName["OnlyMetadata"])
	}
	writeFile(t, orphan, `<DataWeaveResource>`)
	if _, err := LoadProject(p); err == nil {
		t.Fatal("malformed metadata silently used fallback")
	}
}
func TestDataWeaveDependenciesKeepNamespacesAndSourcesSeparate(t *testing.T) {
	local := dataWeaveProject(t, "local", "67.0", "67.0", "local exact source")
	depB := dataWeaveProject(t, "oldB", "66.0", "65.0", "oldB__literal remains unchanged")
	depB.NamespaceRemaps = []namespaceremap.Rule{{From: "oldB", To: "b"}}
	depA := dataWeaveProject(t, "a", "64.0", "63.0", "dependency A source")
	local.ManagedPackageDependencies = []project.ManagedPackageDependency{{Namespace: "oldB", Status: "loaded", Project: &depB}, {Namespace: "a", Status: "loaded", Project: &depA}}
	r, err := LoadProjectWithDependencies(local)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.DataWeaveResources) != 3 {
		t.Fatalf("same-name scripts merged: %+v", r.DataWeaveResources)
	}
	want := []struct{ namespace, source, api string }{{"a", "dependency A source", "63.0"}, {"b", "oldB__literal remains unchanged", "65.0"}, {"local", "local exact source", "67.0"}}
	for i, s := range r.DataWeaveResources {
		if s.Name != "Owned" || s.Namespace != want[i].namespace || s.Content != want[i].source || s.APIVersion != want[i].api {
			t.Fatalf("dependency %d: %+v", i, s)
		}
	}
}
func TestDataWeaveDuplicateSourceFailsInsteadOfDiscarding(t *testing.T) {
	p := dataWeaveProject(t, "", "67.0", "67.0", "original")
	second := filepath.Join(p.Root, "other/main/default/dw/Owned.dwl")
	writeFile(t, second, "different")
	p.DataWeaveFiles = append(p.DataWeaveFiles, second)
	if _, err := LoadProject(p); err == nil {
		t.Fatal("ambiguous same-namespace source discarded")
	}
}
