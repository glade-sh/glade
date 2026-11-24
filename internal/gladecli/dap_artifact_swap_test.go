package gladecli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/startupcache"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestDAPArtifactSwapMatchesStartupIndex(t *testing.T) {
	withoutDAPRetryBackoff(t)
	const artifactA = `{"namespace":"pkg","version":"1.0","sourceHash":"a","apexTypes":[{"kind":"class","name":"Alpha","namespace":"pkg","dependency":true}]}`
	const artifactB = `{"namespace":"pkg","version":"1.0","sourceHash":"b","apexTypes":[{"kind":"class","name":"Bravo","namespace":"pkg","dependency":true}]}`
	hasType := func(index *typesys.Index, name string) bool {
		if index == nil {
			return false
		}
		for _, typ := range index.Types {
			if strings.EqualFold(typ.Name, name) {
				return true
			}
		}
		return false
	}
	for _, swap := range []bool{false, true} {
		t.Run(map[bool]string{false: "steady", true: "swap"}[swap], func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "project")
			writeTestProject(t, root)
			path := filepath.Join(root, "packages", "pkg.glade-package.json")
			writeDriftFile(t, filepath.Join(root, "glade.yml"), "project:\n  managedPackageDependencies: [\"pkg:artifact:packages/pkg.glade-package.json\"]\n")
			writeDriftFile(t, path, artifactB)
			seeded := loadDAPWithDrift(t, root, dapLoadSeams{})
			if seeded.err != nil || !hasType(seeded.index, "Bravo") {
				t.Fatalf("seed index=%v err=%v", hasType(seeded.index, "Bravo"), seeded.err)
			}
			if err := startupcache.Clear(root, startupcache.SubdirDAP); err != nil {
				t.Fatal(err)
			}
			writeDriftFile(t, path, artifactA)
			seams := dapLoadSeams{}
			if swap {
				seams.beforeFrontend = func(a int) {
					if a == 1 {
						writeDriftFile(t, path, artifactB)
					}
				}
				seams.afterFrontend = func(a int) {
					if a == 1 {
						writeDriftFile(t, path, artifactA)
					}
				}
			}
			got := loadDAPWithDrift(t, root, seams)
			if got.err != nil || !hasType(got.index, "Alpha") || hasType(got.index, "Bravo") {
				t.Fatalf("attempts=%d alpha=%v bravo=%v err=%v", got.attempts, hasType(got.index, "Alpha"), hasType(got.index, "Bravo"), got.err)
			}
			want := 1
			if swap {
				want = 2
			}
			if got.attempts != want {
				t.Fatalf("attempts=%d want=%d", got.attempts, want)
			}
			if b, err := os.ReadFile(path); err != nil || string(b) != artifactA {
				t.Fatalf("artifact changed: %v", err)
			}
		})
	}
}
