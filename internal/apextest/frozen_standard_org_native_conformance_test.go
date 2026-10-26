package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func TestFrozenStandardOrgNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/frozen_standard_org"
	cases := readFrozenStandardOrgTSV(t, filepath.Join(fixture, "cases.tsv"))
	native := readFrozenStandardOrgTSV(t, filepath.Join(fixture, "org.tsv"))
	if len(cases) != 5 || strings.Join(cases[0], ",") != "source_api,route,test_class,test_method,behavior" {
		t.Fatalf("incomplete frozen standard org case table: %d rows", len(cases))
	}
	if len(native) != 5 || strings.Join(native[0], ",") != "source_api,route,test_class,test_method,outcome" {
		t.Fatalf("incomplete frozen standard org native table: %d rows", len(native))
	}

	for _, api := range []string{"59.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			want := make(map[string]testreport.Status)
			for _, row := range cases[1:] {
				if len(row) != 5 || row[0] != api || row[1] != "isTest" || row[2] != "FrozenStandardOrgIsolationTest" {
					continue
				}
				want[row[3]] = testreport.StatusUnsupported
			}
			for _, row := range native[1:] {
				if len(row) != 5 || row[0] != api || row[1] != "isTest" || row[2] != "FrozenStandardOrgIsolationTest" {
					continue
				}
				want[row[3]] = testreport.Status(strings.ToLower(row[4]))
			}
			if len(want) != 2 {
				t.Fatalf("expected two native methods at API %s, got %v", api, want)
			}

			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			classDir := filepath.Join(root, "force-app/main/default/classes")
			classPath := filepath.Join(fixture, "project/force-app/main/default/classes/FrozenStandardOrgIsolationTest.cls")
			source, err := os.ReadFile(classPath)
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(classDir, "FrozenStandardOrgIsolationTest.cls"), string(source))
			writeFile(t, filepath.Join(classDir, "FrozenStandardOrgIsolationTest.cls-meta.xml"), "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")

			run := Run(loadTestIndex(t, root), Options{
				SelectedClasses: []string{"FrozenStandardOrgIsolationTest"},
				NoDiskCache:     true,
				Parallelism:     1,
			})
			got := make(map[string]testreport.Status)
			for _, suite := range run.Suites {
				for _, result := range suite.Cases {
					got[result.MethodName] = result.Status
				}
			}
			if len(got) != len(want) {
				t.Fatalf("API %s ran %d methods, want %d; summary=%#v", api, len(got), len(want), run.Summary())
			}
			for method, expected := range want {
				if expected == testreport.StatusUnsupported {
					t.Fatalf("missing native result for %s.%s", "FrozenStandardOrgIsolationTest", method)
				}
				if got[method] != expected {
					t.Errorf("API %s method %s: Glade=%s Salesforce=%s", api, method, got[method], expected)
				}
			}
		})
	}
}

func TestStandardOrgSeedSharesMetadataRecordsWithCopyOnWrite(t *testing.T) {
	seed, template := frozenStandardOrgTemplate(t)
	_ = template.CloneOrg()
	deepClones := testing.AllocsPerRun(3, func() {
		_ = seed.CloneRuntimeFrozenDefinition()
	})
	rowClones := testing.AllocsPerRun(3, func() {
		_ = template.CloneOrg()
	})
	t.Logf("restored org row clone allocations %.0f; deep definition clone allocations %.0f", rowClones, deepClones)
	if rowClones >= deepClones {
		t.Fatalf("restored org row clone allocates %.0f, want fewer than deep definition clone %.0f", rowClones, deepClones)
	}

	first, second := template.CloneOrg(), template.CloneOrg()
	metadata, ok := first.Objects["ApexClass"]
	if !ok || !metadata.RecordsShared || !metadata.IndexesShared {
		t.Fatalf("standard metadata rows are not shared: present=%v recordsShared=%v indexesShared=%v", ok, metadata.RecordsShared, metadata.IndexesShared)
	}
	mutable, cloned := storage.EnsureMutableObjectRecords(&first, "ApexClass")
	if !cloned || mutable == nil {
		t.Fatal("first row did not detach shared metadata rows")
	}
	const markerID = "a0A000000000001"
	mutable.Records[markerID] = storage.Record{Object: "ApexClass", ID: markerID, Fields: map[string]storage.Value{"Name": storage.StringValue("isolated row")}}
	if _, leaked := second.Objects["ApexClass"].Records[markerID]; leaked {
		t.Fatal("metadata row mutation leaked into a sibling row")
	}
	if _, leaked := seed.Objects["ApexClass"].Records[markerID]; leaked {
		t.Fatal("metadata row mutation leaked into the standard org seed")
	}
}

func BenchmarkRestoredStandardOrgClone(b *testing.B) {
	seed, err := orgFromIndexWithError(typesys.Index{})
	if err != nil {
		b.Fatal(err)
	}
	template := vm.NewRestoredRuntimeTemplate(seed, vm.New(nil))
	_ = template.CloneOrg()
	b.Run("deep", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			clone := seed.CloneRuntimeFrozenDefinition()
			if len(clone.Objects) == 0 {
				b.Fatal("expected standard objects")
			}
		}
	})
	b.Run("restored-shared", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			clone := template.CloneOrg()
			if len(clone.Objects) == 0 {
				b.Fatal("expected standard objects")
			}
		}
	})
}

func BenchmarkProjectRuntimeForRequest(b *testing.B) {
	index := typesys.Index{}
	if _, err := NewProjectRuntimeForRequest(index); err != nil {
		b.Fatal(err)
	}
	if err := RegisterProjectRuntimeForRequest(vm.New(nil), index); err != nil {
		b.Fatal(err)
	}
	b.Run("register-compiled-runtime", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := RegisterProjectRuntimeForRequest(vm.New(nil), index); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("cached-template-clone", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := NewProjectRuntimeForRequest(index); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func frozenStandardOrgTemplate(t *testing.T) (storage.OrgState, vm.RestoredRuntimeTemplate) {
	t.Helper()
	seed, err := orgFromIndexWithError(typesys.Index{})
	if err != nil {
		t.Fatal(err)
	}
	return seed, vm.NewRestoredRuntimeTemplate(seed, vm.New(nil))
}

func readFrozenStandardOrgTSV(t *testing.T, path string) [][]string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	return rows
}
