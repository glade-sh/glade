package codeintel_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/codeintel"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func BenchmarkBuildCodeIntelSmallProject(b *testing.B) {
	index := benchmarkCodeIntelIndex(b)
	b.ReportAllocs()
	for b.Loop() {
		graph := codeintel.Build(index)
		if len(graph.Symbols) == 0 {
			b.Fatal("empty graph")
		}
	}
}

func BenchmarkBuildCodeIntelCachedProject(b *testing.B) {
	index := benchmarkCodeIntelIndex(b)
	graph := codeintel.Build(index)
	if err := codeintel.WriteCache(index.Project.Root, graph); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		cached, _, err := codeintel.ReadCache(index.Project.Root)
		if err != nil {
			b.Fatal(err)
		}
		if len(cached.Symbols) == 0 {
			b.Fatal("empty cached graph")
		}
	}
}

func TestCacheFreshTracksBenchmarkFixtureSources(t *testing.T) {
	index := benchmarkCodeIntelIndex(t)
	if err := codeintel.WriteCache(index.Project.Root, codeintel.Build(index)); err != nil {
		t.Fatal(err)
	}
	if !codeintel.CacheFresh(index.Project.Root, index) {
		t.Fatal("cache is not fresh after write")
	}
	path := filepath.Join(index.Project.Root, "force-app", "main", "default", "classes", "Service00.cls")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, []byte("\n// changed\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	if codeintel.CacheFresh(index.Project.Root, index) {
		t.Fatal("cache stayed fresh after source change")
	}
}

func benchmarkCodeIntelIndex(tb testing.TB) typesys.Index {
	tb.Helper()
	root := tb.TempDir()
	var apexFiles []string
	for i := 0; i < 25; i++ {
		name := fmt.Sprintf("Service%02d", i)
		next := fmt.Sprintf("Service%02d", (i+1)%25)
		path := filepath.Join(root, "force-app", "main", "default", "classes", name+".cls")
		source := fmt.Sprintf(`public class %[1]s {
  public static Integer Version = %[2]d;
  public Integer total;
  public void run() {
    Account a = new Account(Name = '%[1]s');
    a.Name = String.valueOf(%[3]s.Version);
    %[3]s helper = new %[3]s();
    total = helper.total;
    List<Account> rows = [SELECT Id, Name FROM Account WHERE Owner.Name != null];
  }
}
`, name, i, next)
		writeBenchmarkFile(tb, path, source)
		apexFiles = append(apexFiles, path)
	}
	objects := []schema.Object{
		{
			Name: "Account",
			Fields: []schema.Field{
				{Name: "Id", Type: "Id"},
				{Name: "Name", Type: "String"},
				{Name: "OwnerId", Type: "Lookup", ReferenceTo: []string{"User"}, RelationshipName: "Owner"},
			},
		},
		{
			Name:   "User",
			Fields: []schema.Field{{Name: "Id", Type: "Id"}, {Name: "Name", Type: "String"}},
		},
	}
	for i := 0; i < 8; i++ {
		objects = append(objects, schema.Object{
			Name:   fmt.Sprintf("Custom%02d__c", i),
			Fields: []schema.Field{{Name: "Id", Type: "Id"}, {Name: "Name", Type: "String"}},
		})
	}
	index := typesys.Build(project.Project{Root: root, ApexFiles: apexFiles}, schema.Schema{Objects: objects})
	if len(index.Diagnostics) > 0 {
		tb.Fatalf("index diagnostics = %#v", index.Diagnostics)
	}
	return index
}

func writeBenchmarkFile(tb testing.TB, path, source string) {
	tb.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		tb.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		tb.Fatal(err)
	}
}
