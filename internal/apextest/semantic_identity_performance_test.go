package apextest

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/semanticcache"
	"github.com/glade-sh/glade/internal/typesys"
)

func semanticIdentityPerformanceFixture(b *testing.B) (typesys.Index, typesys.BuildArtifacts) {
	b.Helper()
	root := b.TempDir()
	paths := make([]string, 3092)
	for i := range paths {
		name := fmt.Sprintf("Class%05d", i)
		paths[i] = filepath.Join(root, name+".cls")
		if err := os.WriteFile(paths[i], []byte("public class "+name+" { String field; }"), 0600); err != nil {
			b.Fatal(err)
		}
	}
	index, artifacts := typesys.BuildWithArtifacts(project.Project{Root: root, ApexFiles: paths}, schema.Schema{})
	for i := 0; i < 15000; i++ {
		index.Types = append(index.Types, typesys.TypeSymbol{Name: fmt.Sprintf("PlatformType%05d", i), Artifact: true, Members: []typesys.MemberSymbol{{Name: "field", Type: "String"}}})
	}
	return index, artifacts
}

func BenchmarkSemanticGateIdentity(b *testing.B) {
	index, artifacts := semanticIdentityPerformanceFixture(b)
	options := sema.AnalyzeOptions{Diagnostics: true, ExportTypes: true, SuppressPerformanceDiagnostics: true, BuildArtifacts: &artifacts}
	want, err := semanticcache.IdentityForBuild(index, &artifacts, options)
	if err != nil {
		b.Fatal(err)
	}
	generation := NewSemanticIdentityGeneration(index, artifacts)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, err := generation.identity(options, false, nil)
		if err != nil || got != want {
			b.Fatal("identity changed", err)
		}
	}
}

func TestSemanticIdentityGenerationViewsAndInputs(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "First.cls")
	second := filepath.Join(root, "Second.cls")
	for _, path := range []string{first, second} {
		if err := os.WriteFile(path, []byte("public class "+pathBase(path)+" {}"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	build := func(objects []schema.Object) (typesys.Index, typesys.BuildArtifacts) {
		return typesys.BuildWithArtifacts(project.Project{Root: root, ApexFiles: []string{first, second}}, schema.Schema{Objects: objects})
	}
	index, artifacts := build(nil)
	options := sema.AnalyzeOptions{Diagnostics: true, ExportTypes: true, BuildArtifacts: &artifacts}
	generation := NewSemanticIdentityGeneration(index, artifacts)
	assertView := func(subset bool, excluded map[string]bool, opts sema.AnalyzeOptions) semanticcache.Identity {
		t.Helper()
		view := index
		if subset {
			view = filterCompileGateIndex(index, excluded)
		}
		want, err := semanticcache.IdentityForBuild(view, &artifacts, opts)
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 2; i++ {
			got, err := generation.identity(opts, subset, excluded)
			if err != nil || got != want {
				t.Fatalf("identity mismatch: got %+v want %+v error %v", got, want, err)
			}
		}
		return want
	}
	full := assertView(false, nil, options)
	one := assertView(true, map[string]bool{first: true}, options)
	two := assertView(true, map[string]bool{second: true}, options)
	if full == one || one == two {
		t.Fatal("views must have distinct identities")
	}
	assertView(false, nil, options)
	changedOptions := options
	changedOptions.ExportTypes = false
	if assertView(false, nil, changedOptions) == full {
		t.Fatal("options must change identity")
	}
	for _, change := range []func(*sema.AnalyzeOptions){
		func(o *sema.AnalyzeOptions) { o.Diagnostics = false },
		func(o *sema.AnalyzeOptions) { o.SuppressPerformanceDiagnostics = true },
	} {
		other := options
		change(&other)
		if assertView(false, nil, other) == full {
			t.Fatal("analysis option did not change identity")
		}
	}
	checkGeneration := func(label string, changed typesys.Index, changedArtifacts typesys.BuildArtifacts) {
		t.Helper()
		next := NewSemanticIdentityGeneration(changed, changedArtifacts)
		got, err := next.identity(options, false, nil)
		if err != nil {
			t.Fatal(label, err)
		}
		want, err := semanticcache.IdentityForBuild(changed, &changedArtifacts, options)
		if err != nil || got != want || got == full {
			t.Fatalf("%s: generation did not preserve changed identity", label)
		}
	}
	changedIndex := index
	changedIndex.Dependencies = []typesys.DependencyInfo{{Namespace: "dependency", Status: "missing"}}
	checkGeneration("dependencies", changedIndex, artifacts)
	changedIndex = index
	changedIndex.Project.Namespace = "newnamespace"
	checkGeneration("project", changedIndex, artifacts)
	projectIndex, projectArtifacts := typesys.BuildWithArtifacts(project.Project{Root: root, Namespace: "newnamespace", ApexFiles: []string{first, second}}, schema.Schema{})
	projectIdentity, err := NewSemanticIdentityGeneration(projectIndex, projectArtifacts).identity(options, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if projectIdentity.DependencySHA256 == full.DependencySHA256 {
		t.Fatal("captured project configuration did not change dependency identity")
	}
	schemaIndex, schemaArtifacts := build([]schema.Object{{Name: "Invoice__c"}})
	checkGeneration("schema", schemaIndex, schemaArtifacts)
	if err := os.WriteFile(first, []byte("public class First { Integer field; }"), 0600); err != nil {
		t.Fatal(err)
	}
	sourceIndex, sourceArtifacts := build(nil)
	checkGeneration("source", sourceIndex, sourceArtifacts)
	if err := os.WriteFile(first+"-meta.xml", []byte(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>`), 0600); err != nil {
		t.Fatal(err)
	}
	metadataIndex, metadataArtifacts := build(nil)
	sourceGeneration := NewSemanticIdentityGeneration(sourceIndex, sourceArtifacts)
	sourceIdentity, err := sourceGeneration.identity(options, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	metadataIdentity, err := NewSemanticIdentityGeneration(metadataIndex, metadataArtifacts).identity(options, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if sourceIdentity == metadataIdentity {
		t.Fatal("metadata must change identity")
	}
	if err := os.WriteFile(first+"-meta.xml", []byte(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>68.0</apiVersion><status>Active</status></ApexClass>`), 0600); err != nil {
		t.Fatal(err)
	}
	updatedIndex, updatedArtifacts := build(nil)
	updatedIdentity, err := NewSemanticIdentityGeneration(updatedIndex, updatedArtifacts).identity(options, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if metadataIdentity == updatedIdentity {
		t.Fatal("metadata content must change identity")
	}
}

func pathBase(path string) string { return strings.TrimSuffix(filepath.Base(path), ".cls") }

func TestSemanticIdentityGenerationRejectsIncompleteAndVolatile(t *testing.T) {
	invalid := NewSemanticIdentityGeneration(typesys.Index{}, typesys.BuildArtifacts{})
	for i := 0; i < 2; i++ {
		if _, err := invalid.identity(sema.AnalyzeOptions{}, false, nil); err == nil {
			t.Fatal("incomplete generation returned an identity")
		}
	}
	root := t.TempDir()
	path := filepath.Join(root, "Draft.cls")
	if err := os.WriteFile(path, []byte("public class Draft {}"), 0600); err != nil {
		t.Fatal(err)
	}
	p := project.Project{Root: root, ApexFiles: []string{path}}
	overlay := typesys.NewSourceOverlay(map[string]string{path: "public class Draft { Integer field; }"})
	index, artifacts := typesys.BuildWithArtifactsOverlay(p, schema.Schema{}, overlay)
	generation := NewSemanticIdentityGeneration(index, artifacts)
	for i := 0; i < 2; i++ {
		if _, err := generation.identity(sema.AnalyzeOptions{}, false, nil); !errors.Is(err, typesys.ErrVolatileGeneration) {
			t.Fatalf("volatile identity error: %v", err)
		}
	}
}

func TestSemanticIdentityGenerationRejectsLiveEditAfterMemoHit(t *testing.T) {
	restore := EnableDiskCacheForTesting()
	t.Cleanup(restore)
	t.Cleanup(InvalidateRuntimeCaches)
	InvalidateRuntimeCaches()
	root := t.TempDir()
	path := filepath.Join(root, "ExampleTest.cls")
	original := `@IsTest private class ExampleTest { @IsTest static void passes() { System.assertEquals(1, 1); } }`
	changed := strings.Replace(original, "Equals(1, 1)", "Equals(2, 2)", 1)
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	index, artifacts := typesys.BuildWithArtifacts(project.Project{Root: root, ApexFiles: []string{path}}, schema.Schema{})
	ctx := WithSemanticIdentityGeneration(context.Background(), NewSemanticIdentityGeneration(index, artifacts))
	options := Options{BuildArtifacts: &artifacts}
	cases := Discover(index, options)
	first := RunCasesContext(ctx, index, options, cases)
	if first.Summary().Passed != 1 {
		t.Fatalf("initial run: %+v", first.Summary())
	}
	raced := runCasesContextWithSemanticGateHooks(ctx, index, options, cases, semanticGateHooks{afterIdentity: func() {
		if err := os.WriteFile(path, []byte(changed), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
			t.Fatal(err)
		}
	}})
	if !strings.Contains(firstRunProblem(raced), "source snapshot mismatch") {
		t.Fatalf("live edit: %s", firstRunProblem(raced))
	}
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	again := RunCasesContext(ctx, index, options, cases)
	if again.Summary().Passed != 1 {
		t.Fatalf("restored run: %+v %s", again.Summary(), firstRunProblem(again))
	}
}
