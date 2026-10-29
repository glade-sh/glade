package visualforce

import (
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestDependencyAccessNativeConformance(t *testing.T) {
	runDependencyAccessNativeConformance(t, "dependency_access", 16, 8)
}

func TestNamespacedDependencyAccessNativeConformance(t *testing.T) {
	runDependencyAccessNativeConformance(t, "dependency_access_namespaced", 24, 12)
}

func runDependencyAccessNativeConformance(t *testing.T, name string, wantRows, wantCases int) {
	t.Helper()
	fixture := filepath.Join("testdata", name)
	read := func(name string) [][]string {
		t.Helper()
		f, err := os.Open(filepath.Join(fixture, name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		r := csv.NewReader(f)
		r.Comma = '\t'
		rows, err := r.ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		return rows[1:]
	}
	cases := map[string][]string{}
	for _, row := range read("cases.tsv") {
		cases[row[0]] = row
	}
	rows := read("org.tsv")
	if len(rows) != wantRows || len(cases) != wantCases {
		t.Fatalf("native rows=%d cases=%d", len(rows), len(cases))
	}
	for _, row := range rows {
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS(filepath.Join(fixture, "project"))); err != nil {
				t.Fatal(err)
			}
			if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if entry.IsDir() {
					return nil
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				return os.WriteFile(path, []byte(strings.ReplaceAll(string(data), "62.0", row[0])), 0644)
			}); err != nil {
				t.Fatal(err)
			}
			c, ok := cases[row[1]]
			if !ok {
				t.Fatalf("unknown native case %s", row[1])
			}
			entries := ""
			if c[2] != "" {
				entries += fmt.Sprintf("<classAccesses><apexClass>%s</apexClass><enabled>%s</enabled></classAccesses>", c[2], c[4])
			}
			if c[3] != "" {
				entries += fmt.Sprintf("<pageAccesses><apexPage>%s</apexPage><enabled>%s</enabled></pageAccesses>", c[3], c[4])
			}
			path := filepath.Join(root, "consumer/src/permissionsets/Access.permissionset-meta.xml")
			if c[1] == "Profile" {
				path = filepath.Join(root, "consumer/src/profiles/Standard.profile-meta.xml")
			} else {
				entries += "<label>Dependency access probe</label>"
			}
			writeFile(t, path, fmt.Sprintf(`<%s xmlns="http://soap.sforce.com/2006/04/metadata">%s</%s>`, c[1], entries, c[1]))
			p, err := project.Load(filepath.Join(root, "consumer"))
			if err != nil {
				t.Fatal(err)
			}
			if len(p.ManagedPackageDependencies) != 1 || p.ManagedPackageDependencies[0].Status != "loaded" {
				t.Fatalf("dependency not loaded: %+v", p.ManagedPackageDependencies)
			}
			check := func(p project.Project) {
				t.Helper()
				err := ValidateVisualforceAccessMetadata(p)
				if row[2] == "DEPLOY_OK" {
					if err != nil {
						t.Fatalf("native accepted access: %v", err)
					}
				} else {
					var access *VisualforceAccessMetadataError
					if !errors.As(err, &access) || access.Message != row[3] || access.File != path {
						t.Fatalf("access error=%v, want %s at %s", err, row[3], path)
					}
				}
				_, err = LoadProject(p)
				if (err == nil) != (row[2] == "DEPLOY_OK") {
					t.Fatalf("strict loader error=%v, native=%s", err, row[2])
				}
				var want []string
				if row[2] == "DEPLOY_OK" {
					want = []string{"ConsumerAccessPage"}
				}
				if got := PageNames(p); !slices.Equal(got, want) {
					t.Fatalf("PageNames=%v want %v", got, want)
				}
			}
			check(p)
			if c[1] == "PermissionSet" && row[2] == "DEPLOY_OK" && row[1] != "local_access" {
				// An installed transitive dependency is visible too. Ignore unloaded
				// views even if they retain a project pointer; do not read absent sources.
				dep := p.ManagedPackageDependencies[0]
				if dep.Namespace != "" {
					dep.Project.Namespace = ""
					check(p) // The dependency edge owns its effective namespace.
					unqualified := dep
					unqualified.Namespace = ""
					p.ManagedPackageDependencies = []project.ManagedPackageDependency{unqualified, dep}
					check(p) // A shared project pointer must retain both views.
					dep.Project.ManagedPackageDependencies = []project.ManagedPackageDependency{dep}
					check(p) // Repeated loaded views terminate even in a cycle.
					dep.Project.ManagedPackageDependencies = nil
				}
				p.ManagedPackageDependencies = []project.ManagedPackageDependency{{Status: "loaded", Project: &project.Project{ManagedPackageDependencies: []project.ManagedPackageDependency{dep}}}}
				check(p)
				p.ManagedPackageDependencies = []project.ManagedPackageDependency{dep}
				p.ManagedPackageDependencies[0].Status = "missing"
				if err := ValidateVisualforceAccessMetadata(p); err == nil {
					t.Fatal("unloaded dependency granted access")
				}
				p.ManagedPackageDependencies[0].Status = "loaded"
				p.ManagedPackageDependencies[0].Project = nil
				if err := ValidateVisualforceAccessMetadata(p); err == nil {
					t.Fatal("artifact-only dependency granted source access")
				}
			}
		})
	}
}
