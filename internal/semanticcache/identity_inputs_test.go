package semanticcache

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/namespaceremap"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestIdentityForBuildIncludesAdditionalGenerationInputs(t *testing.T) {
	root := t.TempDir()
	classFile := filepath.Join(root, "Input.cls")
	triggerFile := filepath.Join(root, "InputTrigger.trigger")
	if err := os.WriteFile(classFile, []byte("public class Input {}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(triggerFile, []byte("trigger InputTrigger on Account (before insert) { Integer x = 1; }"), 0600); err != nil {
		t.Fatal(err)
	}
	p := project.Project{Root: root, ApexFiles: []string{classFile, triggerFile}}
	build := func(p project.Project) (typesys.Index, typesys.BuildArtifacts, Identity) {
		t.Helper()
		index, artifacts := typesys.BuildWithArtifacts(p, schema.Schema{})
		identity, err := IdentityForBuild(index, &artifacts, sema.AnalyzeOptions{Diagnostics: true})
		if err != nil {
			t.Fatal(err)
		}
		return index, artifacts, identity
	}
	index, artifacts, base := build(p)
	if len(index.Triggers) == 0 {
		t.Fatal("fixture did not capture trigger")
	}
	classIndex := -1
	for i, typ := range index.Types {
		if typ.File == classFile {
			classIndex = i
			break
		}
	}
	if classIndex < 0 {
		t.Fatal("fixture did not capture class")
	}
	occurrences := []struct {
		name   string
		change func(*typesys.TypeSymbol)
	}{
		{"name", func(typ *typesys.TypeSymbol) { typ.Name += "Changed" }},
		{"file", func(typ *typesys.TypeSymbol) { typ.File += ".changed" }},
		{"root", func(typ *typesys.TypeSymbol) { typ.SourceRoot += ".changed" }},
		{"namespace", func(typ *typesys.TypeSymbol) { typ.Namespace += "changed" }},
		{"version", func(typ *typesys.TypeSymbol) { typ.Version += "changed" }},
		{"dependency", func(typ *typesys.TypeSymbol) { typ.Dependency = !typ.Dependency }},
		{"remapping", func(typ *typesys.TypeSymbol) {
			typ.SourceNamespaceRemaps = []namespaceremap.Rule{{From: "alpha", To: "beta"}}
		}},
	}
	for _, occurrence := range occurrences {
		t.Run(occurrence.name, func(t *testing.T) {
			changed := index
			changed.Types = append([]typesys.TypeSymbol(nil), index.Types...)
			occurrence.change(&changed.Types[classIndex])
			got, err := IdentityForBuild(changed, &artifacts, sema.AnalyzeOptions{Diagnostics: true})
			if err == nil && got == base {
				t.Fatal("changed occurrence retained the original identity")
			}
		})
	}
	check := func(label string, index typesys.Index) {
		t.Helper()
		got, err := IdentityForBuild(index, &artifacts, sema.AnalyzeOptions{Diagnostics: true})
		if err != nil {
			t.Fatal(err)
		}
		if got == base {
			t.Fatalf("%s retained the original identity", label)
		}
	}
	metadata := index
	metadata.CustomMetadataRecords = []schema.CustomMetadataRecord{{FullName: "Setting.Item", ObjectName: "Setting", DeveloperName: "Item"}}
	check("custom metadata", metadata)
	diagnostics := index
	diagnostics.Diagnostics = []diagnostic.Diagnostic{{Message: "changed"}}
	check("index diagnostics", diagnostics)
	sourceVersion := p
	sourceVersion.SourceAPIVersion = "67.0"
	_, _, versionIdentity := build(sourceVersion)
	if versionIdentity == base {
		t.Fatal("source API version retained the original identity")
	}
	remapped := p
	remapped.NamespaceRemaps = []namespaceremap.Rule{{From: "alpha", To: "beta"}}
	_, _, remappedIdentity := build(remapped)
	if remappedIdentity == base {
		t.Fatal("namespace remapping retained the original identity")
	}
	if err := os.WriteFile(triggerFile, []byte("trigger InputTrigger on Account (before insert) { Integer x = 2; }"), 0600); err != nil {
		t.Fatal(err)
	}
	_, _, triggerIdentity := build(p)
	if triggerIdentity == base {
		t.Fatal("trigger edit retained the original identity")
	}
}
