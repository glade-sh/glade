package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestCacheStaticEntryAcrossProjectSources(t *testing.T) {
	for _, api := range []string{"62.0", "66.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS("../vm/testdata/cache_static_dispatch/project")); err != nil {
				t.Fatal(err)
			}
			if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
				if walkErr != nil || entry.IsDir() {
					return walkErr
				}
				if entry.Name() != "sfdx-project.json" && !strings.HasSuffix(path, ".cls-meta.xml") {
					return nil
				}
				source, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				return os.WriteFile(path, []byte(strings.ReplaceAll(string(source), "66.0", api)), 0600)
			}); err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			if index.Project.SourceAPIVersion != api {
				t.Fatalf("source API=%s, want %s", index.Project.SourceAPIVersion, api)
			}
			// Model the local org's authored default separately from the
			// namespaced project sources being compiled by this test.
			partition := filepath.Join(root, "default.cachePartition-meta.xml")
			metadata := `<PlatformCachePartition xmlns="http://soap.sforce.com/2006/04/metadata"><isDefaultPartition>true</isDefaultPartition><masterLabel>default</masterLabel><platformCachePartitionTypes><allocatedCapacity>1</allocatedCapacity><cacheType>Organization</cacheType></platformCachePartitionTypes><platformCachePartitionTypes><allocatedCapacity>1</allocatedCapacity><cacheType>Session</cacheType></platformCachePartitionTypes></PlatformCachePartition>`
			if err := os.WriteFile(partition, []byte(metadata), 0600); err != nil {
				t.Fatal(err)
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			p.ManagedPackageDependencies = append(p.ManagedPackageDependencies, project.ManagedPackageDependency{
				Status: "loaded", Project: &project.Project{Root: root, CachePartitionFiles: []string{partition}},
			})
			run := Run(index, Options{NoDiskCache: true, Parallelism: 1, ProjectSnapshot: &p})
			if problem := firstRunProblem(run); strings.Contains(problem, "A default partition was not found") {
				t.Fatalf("project cache operations require a configured default partition: %s", problem)
			}
			if got := run.Summary(); got.Total != 4 || got.Passed != 4 {
				t.Fatalf("summary=%+v, problem=%s", got, firstRunProblem(run))
			}
		})
	}
}
