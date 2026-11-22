package frontendcache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func editFixture(tb testing.TB, count int) (project.Project, schema.Schema, string) {
	tb.Helper()
	root := tb.TempDir()
	dir := filepath.Join(root, "force-app", "main", "default", "classes")
	if err := os.MkdirAll(dir, 0700); err != nil {
		tb.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sfdx-project.json"), []byte(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`), 0600); err != nil {
		tb.Fatal(err)
	}
	for i := 0; i < count; i++ {
		name := fmt.Sprintf("Item%04d", i)
		if err := os.WriteFile(filepath.Join(dir, name+".cls"), []byte("public class "+name+" { Integer value; }"), 0600); err != nil {
			tb.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name+".cls-meta.xml"), []byte("<ApexClass><apiVersion>67.0</apiVersion></ApexClass>"), 0600); err != nil {
			tb.Fatal(err)
		}
	}
	p, err := project.Load(root)
	if err != nil {
		tb.Fatal(err)
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		tb.Fatal(err)
	}
	return p, s, filepath.Join(dir, "Item0000.cls")
}

func TestEditedFrontendMatchesFreshGeneration(t *testing.T) {
	p, s, path := editFixture(t, 20)
	build := func() (typesys.Index, typesys.BuildArtifacts) { return typesys.BuildWithArtifacts(p, s) }
	LoadOrBuild(p, s, build)
	if err := os.WriteFile(path, []byte("public class Item0000 { Integer changed; }"), 0600); err != nil {
		t.Fatal(err)
	}
	got := LoadOrBuild(p, s, build)
	if !got.Reused || got.Hit {
		t.Fatalf("edit did not use incremental frontend: %#v", got)
	}
	fresh, _ := build()
	a, err := json.Marshal(got.Index)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(fresh)
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != string(b) {
		t.Fatal("incremental index differs from fresh")
	}
	if err := typesys.ValidateBuildGeneration(got.Index, &got.Artifacts); err != nil {
		t.Fatal(err)
	}
}

func BenchmarkEditedFrontend1500Classes(b *testing.B) {
	p, s, path := editFixture(b, 1500)
	build := func() (typesys.Index, typesys.BuildArtifacts) { return typesys.BuildWithArtifacts(p, s) }
	LoadOrBuild(p, s, build)
	b.ReportAllocs()
	var last Result
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := os.WriteFile(path, []byte(fmt.Sprintf("public class Item0000 { Integer value%d; }", i%2)), 0600); err != nil {
			b.Fatal(err)
		}
		last = LoadOrBuild(p, s, build)
		if !last.Reused || last.Hit {
			b.Fatal("incremental frontend was not reused")
		}
	}
	b.StopTimer()
	fresh, _ := build()
	gotJSON, err := json.Marshal(last.Index)
	if err != nil {
		b.Fatal(err)
	}
	wantJSON, err := json.Marshal(fresh)
	if err != nil {
		b.Fatal(err)
	}
	if string(gotJSON) != string(wantJSON) {
		b.Fatal("incremental index differs from fresh")
	}
}
