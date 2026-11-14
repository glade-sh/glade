package gladecli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

type dapBenchmarkSize struct {
	name                    string
	classes, objects, pages int
}

var dapBenchmarkSizes = []dapBenchmarkSize{{"mid", 150, 40, 8}, {"large", 800, 160, 24}}

func makeDAPBenchmarkProject(b testing.TB, size dapBenchmarkSize) string {
	b.Helper()
	root := b.TempDir()
	write := func(path, text string) {
		b.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			b.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0644); err != nil {
			b.Fatal(err)
		}
	}
	write(filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"src","default":true}],"sourceApiVersion":"62.0"}`)
	for i := 0; i < size.classes; i++ {
		name := fmt.Sprintf("BenchClass%04d", i)
		path := filepath.Join(root, "src", "classes", name+".cls")
		write(path, fmt.Sprintf("public class %s { public static Integer value() { return %d; } }\n", name, i))
		write(path+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>`)
	}
	for i := 0; i < size.objects; i++ {
		name := fmt.Sprintf("BenchObject%04d__c", i)
		path := filepath.Join(root, "src", "objects", name, name+".object-meta.xml")
		write(path, fmt.Sprintf(`<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Bench %d</label><pluralLabel>Benches</pluralLabel><nameField><label>Name</label><type>Text</type></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>Private</sharingModel></CustomObject>`, i))
	}
	for i := 0; i < size.pages; i++ {
		name := fmt.Sprintf("BenchPage%04d", i)
		path := filepath.Join(root, "src", "pages", name+".page")
		write(path, `<apex:page>benchmark</apex:page>`)
		write(path+"-meta.xml", fmt.Sprintf(`<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><label>%s</label></ApexPage>`, name))
	}
	return root
}

// BenchmarkDAPStartupWarm exercises manifest hashing and pinned directory
// verification on projects with Apex sources, Visualforce pages and metadata.
func BenchmarkDAPStartupWarm(b *testing.B) {
	for _, size := range dapBenchmarkSizes {
		b.Run(size.name, func(b *testing.B) {
			root := makeDAPBenchmarkProject(b, size)
			if _, _, _, _, err := loadDAPStartupStateWithIndex(context.Background(), root); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _, _, index, err := loadDAPStartupStateWithIndex(context.Background(), root)
				if err != nil {
					b.Fatal(err)
				}
				if index != nil {
					b.Fatal("expected a warm startup cache")
				}
			}
		})
	}
}

// BenchmarkDAPStartupCold uses a new root on every iteration so no runtime or
// disk entry from a previous iteration can satisfy the load.
func BenchmarkDAPStartupCold(b *testing.B) {
	for _, size := range dapBenchmarkSizes {
		b.Run(size.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				root := makeDAPBenchmarkProject(b, size)
				b.StartTimer()
				_, _, _, index, err := loadDAPStartupStateWithIndex(context.Background(), root)
				if err != nil {
					b.Fatal(err)
				}
				if index == nil {
					b.Fatal("expected a cold build")
				}
			}
		})
	}
}

// TestDAPBenchmarkFixtureOutcome allows the same fixture's served org and
// runtime to be compared between the base and candidate, independent of root.
func TestDAPBenchmarkFixtureOutcome(t *testing.T) {
	for _, size := range dapBenchmarkSizes {
		t.Run(size.name, func(t *testing.T) {
			root := makeDAPBenchmarkProject(t, size)
			org, runtime, api, index, err := loadDAPStartupStateWithIndex(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			if index == nil || len(runtime.PageNames) != size.pages || len(runtime.Methods) < size.classes {
				t.Fatalf("index=%v pages=%d methods=%d", index != nil, len(runtime.PageNames), len(runtime.Methods))
			}
			body, err := json.Marshal(struct {
				Org     any
				Runtime any
				API     string
			}{org, compiledRuntimeToCache(runtime), api})
			if err != nil {
				t.Fatal(err)
			}
			body = bytes.ReplaceAll(body, []byte(root), []byte("<ROOT>"))
			fmt.Printf("benchmark-outcome %s %x\n", size.name, sha256.Sum256(body))
		})
	}
}
