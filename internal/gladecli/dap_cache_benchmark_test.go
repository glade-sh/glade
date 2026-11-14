package gladecli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

type dapBenchmarkSize struct {
	name             string
	classes, objects int
}

var dapBenchmarkSizes = []dapBenchmarkSize{{"mid", 150, 40}, {"large", 800, 160}}

func makeDAPBenchmarkProject(b *testing.B, size dapBenchmarkSize) string {
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
	return root
}

// BenchmarkDAPStartupWarm exercises manifest hashing and pinned directory
// verification on projects with many Apex sources and metadata directories.
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
