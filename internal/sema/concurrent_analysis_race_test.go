package sema

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// TestAnalyzeWithOptionsConcurrentOnSharedIndex runs semantic analysis from
// several goroutines on one typesys.Index. A shared or cached index is read by
// concurrent callers in the product, so the analyzer must not write to it.
// Run under -race.
func TestAnalyzeWithOptionsConcurrentOnSharedIndex(t *testing.T) {
	root := t.TempDir()
	classes := map[string]string{
		"Helper": `public class Helper {
    public static Integer twice(Integer n) { return n * 2; }
    public Account build(String name) { Account a = new Account(Name = name); return a; }
}`,
		"Worker": `public class Worker {
    public List<Account> run(Set<Id> ids) {
        List<Account> rows = [SELECT Id, Name FROM Account WHERE Id IN :ids];
        Map<Id, Account> byId = new Map<Id, Account>(rows);
        for (Account a : rows) { a.Name = Helper.twice(1) + a.Name; }
        Helper h = new Helper();
        rows.add(h.build('x'));
        insert rows;
        return rows;
    }
}`,
		"WorkerTest": `@IsTest
private class WorkerTest {
    @IsTest static void works() {
        Worker w = new Worker();
        System.assertEquals(2, Helper.twice(1));
        w.run(new Set<Id>());
    }
}`,
	}
	var files []string
	for name, body := range classes {
		path := filepath.Join(root, name+".cls")
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		files = append(files, path)
	}
	index := typesys.Build(project.Project{Root: root, SourceAPIVersion: "67.0", ApexFiles: files}, schema.Schema{Objects: []schema.Object{{Name: "Account", Fields: []schema.Field{{Name: "Name", Type: "Text"}}}}})
	opts := AnalyzeOptions{Diagnostics: true, ExportTypes: true, SuppressPerformanceDiagnostics: true}
	const workers = 8
	results := make([][]diagnostic.Diagnostic, workers)
	var wg sync.WaitGroup
	for i := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = AnalyzeWithOptions(index, opts).Diagnostics
		}()
	}
	wg.Wait()
	for i := 1; i < workers; i++ {
		if len(results[i]) != len(results[0]) {
			t.Fatalf("worker %d diagnostics = %d, worker 0 = %d", i, len(results[i]), len(results[0]))
		}
	}
}

// TestAnalyzeAnonymousConcurrentOnSharedIndex covers the server path: each
// HTTP request analyzes anonymous Apex against the one shared project index.
func TestAnalyzeAnonymousConcurrentOnSharedIndex(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Helper.cls")
	if err := os.WriteFile(path, []byte("public class Helper { public static Integer twice(Integer n) { return n * 2; } public Account build(String name) { return new Account(Name = name); } }"), 0o600); err != nil {
		t.Fatal(err)
	}
	index := typesys.Build(project.Project{Root: root, SourceAPIVersion: "67.0", ApexFiles: []string{path}}, schema.Schema{Objects: []schema.Object{{Name: "Account", Fields: []schema.Field{{Name: "Name", Type: "Text"}}}}})
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			AnalyzeAnonymous(index, "Account a = new Helper().build('x'); Integer n = Helper.twice(2); insert a;", "67.0")
		}()
	}
	wg.Wait()
}

func BenchmarkAnalyzeSharedIndexSingleThread(b *testing.B) {
	root := b.TempDir()
	var files []string
	for i := range 40 {
		path := filepath.Join(root, "C"+string(rune('A'+i/26))+string(rune('a'+i%26))+".cls")
		body := "public class " + strings.TrimSuffix(filepath.Base(path), ".cls") + " { public Integer f(Integer n, List<Account> rows) { for (Account a : rows) { a.Name = 'x' + a.Description; } return n * 2; } }"
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			b.Fatal(err)
		}
		files = append(files, path)
	}
	index := typesys.Build(project.Project{Root: root, SourceAPIVersion: "67.0", ApexFiles: files}, schema.Schema{Objects: []schema.Object{{Name: "Account", Fields: []schema.Field{{Name: "Name", Type: "Text"}}}}})
	opts := AnalyzeOptions{Diagnostics: true, SuppressPerformanceDiagnostics: true}
	b.ResetTimer()
	for range b.N {
		AnalyzeWithOptions(index, opts)
	}
}

// TestEnrichSchemaDerivedObjectsConcurrentOnSharedIndex covers the in-place
// field merges in semaEnrichSchemaObject. The objects below have fields that
// the merge rewrites (empty Type, existing SetupOwnerId/RecordTypeId) and spare
// slice capacity that an append would write into. Run under -race.
func TestEnrichSchemaDerivedObjectsConcurrentOnSharedIndex(t *testing.T) {
	newFields := func(fields ...schema.Field) []schema.Field {
		out := make([]schema.Field, len(fields), len(fields)+8)
		copy(out, fields)
		return out
	}
	index := typesys.Index{Objects: []schema.Object{
		{Name: "Meta__mdt", Fields: newFields(schema.Field{Name: "Label"}, schema.Field{Name: "DeveloperName"})},
		{Name: "Setting__c", CustomSettingsType: "Hierarchy", Fields: newFields(schema.Field{Name: "SetupOwnerId", Type: "Text"})},
		{Name: "Typed__c", Fields: newFields(schema.Field{Name: "RecordTypeId", Type: "Text"})},
	}}
	var before [][]schema.Field
	for _, o := range index.Objects {
		before = append(before, append([]schema.Field(nil), o.Fields...))
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			enrichIndexWithSchemaDerivedObjects(index)
		}()
	}
	wg.Wait()
	for i, o := range index.Objects {
		if len(o.Fields) != len(before[i]) {
			t.Fatalf("%s fields length changed", o.Name)
		}
		for j := range o.Fields {
			if o.Fields[j].Type != before[i][j].Type || len(o.Fields[j].ReferenceTo) != len(before[i][j].ReferenceTo) {
				t.Fatalf("%s field %s mutated: %+v", o.Name, o.Fields[j].Name, o.Fields[j])
			}
		}
	}
}
