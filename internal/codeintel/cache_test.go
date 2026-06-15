package codeintel_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/codeintel"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestCacheWriteReadPreservesSymbolsAndUses(t *testing.T) {
	root := t.TempDir()
	classPath := filepath.Join("force-app", "main", "default", "classes", "InvoiceService.cls")
	writeCacheTestFile(t, filepath.Join(root, classPath), "public class InvoiceService {}\n")
	graph := codeintel.NewGraph(root)
	graph.AddSymbol(codeintel.Symbol{
		ID:   codeintel.ApexTypeID("", "InvoiceService"),
		Kind: codeintel.SymbolApexType,
		Name: "InvoiceService",
		File: classPath,
		Range: diagnostic.Range{
			Start: diagnostic.Position{Line: 1, Column: 14},
			End:   diagnostic.Position{Line: 1, Column: 28},
		},
	})
	graph.AddUse(codeintel.Use{
		SymbolID: codeintel.ApexTypeID("", "InvoiceService"),
		Kind:     codeintel.UseDeclaration,
		Name:     "InvoiceService",
		File:     classPath,
		Range: diagnostic.Range{
			Start: diagnostic.Position{Line: 1, Column: 14},
			End:   diagnostic.Position{Line: 1, Column: 28},
		},
		Resolved: true,
	})

	if err := codeintel.WriteCache(root, graph); err != nil {
		t.Fatalf("WriteCache: %v", err)
	}
	got, meta, err := codeintel.ReadCache(root)
	if err != nil {
		t.Fatalf("ReadCache: %v", err)
	}

	if meta.SchemaVersion == 0 {
		t.Fatal("schema version not recorded")
	}
	if meta.ProjectRoot != root {
		t.Fatalf("project root = %q, want %q", meta.ProjectRoot, root)
	}
	if meta.SourceHash == "" {
		t.Fatal("source hash not recorded")
	}
	if meta.CreatedAt.IsZero() {
		t.Fatal("created at not recorded")
	}
	if got.ProjectRoot != graph.ProjectRoot {
		t.Fatalf("project root = %q, want %q", got.ProjectRoot, graph.ProjectRoot)
	}
	if got.Symbols[codeintel.ApexTypeID("", "InvoiceService")].Name != "InvoiceService" {
		t.Fatalf("symbols = %#v", got.Symbols)
	}
	if len(got.Uses) != 1 || got.Uses[0].Name != "InvoiceService" || got.Uses[0].Kind != codeintel.UseDeclaration {
		t.Fatalf("uses = %#v", got.Uses)
	}
}

func TestCacheFreshReturnsFalseForStaleSourceHash(t *testing.T) {
	root := t.TempDir()
	classPath := filepath.Join("force-app", "main", "default", "classes", "InvoiceService.cls")
	writeCacheTestFile(t, filepath.Join(root, classPath), "public class InvoiceService {}\n")
	graph := codeintel.NewGraph(root)
	graph.AddSymbol(codeintel.Symbol{
		ID:   codeintel.ApexTypeID("", "InvoiceService"),
		Kind: codeintel.SymbolApexType,
		Name: "InvoiceService",
		File: classPath,
	})
	index := typesys.Index{
		Project: typesys.ProjectInfo{Root: root},
		Types: []typesys.TypeSymbol{{
			Kind: apexast.DeclarationClass,
			Name: "InvoiceService",
			File: classPath,
		}},
	}

	if err := codeintel.WriteCache(root, graph); err != nil {
		t.Fatalf("WriteCache: %v", err)
	}
	if !codeintel.CacheFresh(root, index) {
		t.Fatal("cache should be fresh before source changes")
	}
	writeCacheTestFile(t, filepath.Join(root, classPath), "public class InvoiceService { void run() {} }\n")

	if codeintel.CacheFresh(root, index) {
		t.Fatal("cache should be stale after source changes")
	}
}

func TestCacheFreshReturnsFalseForSchemaShapeChange(t *testing.T) {
	root := t.TempDir()
	initial := schema.Schema{Objects: []schema.Object{{
		Name:   "Account",
		Fields: []schema.Field{{Name: "Name", Type: "String"}},
	}}}
	if err := codeintel.WriteSchemaCache(root, initial); err != nil {
		t.Fatalf("WriteSchemaCache: %v", err)
	}
	freshIndex := typesys.Index{
		Project: typesys.ProjectInfo{Root: root},
		Objects: initial.Objects,
	}
	if !codeintel.CacheFresh(root, freshIndex) {
		t.Fatal("schema cache should be fresh for matching schema")
	}
	staleIndex := typesys.Index{
		Project: typesys.ProjectInfo{Root: root},
		Objects: []schema.Object{{
			Name:   "Account",
			Fields: []schema.Field{{Name: "Name", Type: "String"}, {Name: "Industry", Type: "String"}},
		}},
	}
	if codeintel.CacheFresh(root, staleIndex) {
		t.Fatal("schema cache should be stale after schema shape changes")
	}
}

func TestReadCacheMissingReturnsErrCacheMiss(t *testing.T) {
	_, _, err := codeintel.ReadCache(t.TempDir())
	if !errors.Is(err, codeintel.ErrCacheMiss) {
		t.Fatalf("ReadCache error = %v, want ErrCacheMiss", err)
	}
}

func TestClearCacheRemovesOnlySymbolsDirectory(t *testing.T) {
	root := t.TempDir()
	symbolFile := filepath.Join(codeintel.CacheDir(root), "index.json")
	testCacheFile := filepath.Join(root, ".glade", "test", "startup.gob")
	dapCacheFile := filepath.Join(root, ".glade", "dap", "startup.json")
	writeCacheTestFile(t, symbolFile, "{}")
	writeCacheTestFile(t, testCacheFile, "test-cache")
	writeCacheTestFile(t, dapCacheFile, "dap-cache")

	if err := codeintel.ClearCache(root); err != nil {
		t.Fatalf("ClearCache: %v", err)
	}

	if _, err := os.Stat(codeintel.CacheDir(root)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("symbols dir stat err = %v, want not exist", err)
	}
	if _, err := os.Stat(testCacheFile); err != nil {
		t.Fatalf("test cache was removed: %v", err)
	}
	if _, err := os.Stat(dapCacheFile); err != nil {
		t.Fatalf("dap cache was removed: %v", err)
	}
}

func writeCacheTestFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}
