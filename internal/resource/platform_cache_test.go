package resource

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func writePlatformCachePartition(t *testing.T, root, name string, isDefault bool, capacity int) string {
	t.Helper()
	metadata := fmt.Sprintf(`<PlatformCachePartition xmlns="http://soap.sforce.com/2006/04/metadata">
<isDefaultPartition>%t</isDefaultPartition><masterLabel>%s</masterLabel>
<platformCachePartitionTypes><allocatedCapacity>%d</allocatedCapacity><allocatedPartnerCapacity>0</allocatedPartnerCapacity><allocatedPurchasedCapacity>0</allocatedPurchasedCapacity><allocatedTrialCapacity>0</allocatedTrialCapacity><cacheType>Organization</cacheType></platformCachePartitionTypes>
<platformCachePartitionTypes><allocatedCapacity>%d</allocatedCapacity><allocatedPartnerCapacity>0</allocatedPartnerCapacity><allocatedPurchasedCapacity>0</allocatedPurchasedCapacity><allocatedTrialCapacity>0</allocatedTrialCapacity><cacheType>Session</cacheType></platformCachePartitionTypes>
</PlatformCachePartition>`, isDefault, name, capacity, capacity)
	path := filepath.Join(root, "force-app/main/default/cachePartitions", name+".cachePartition-meta.xml")
	writeFile(t, path, metadata)
	return path
}

func TestProjectPlatformCachePartitions(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	// A42 R001/R003, R208/R209 and N001/N002: captured default and
	// zero-capacity metadata, with non-trial Org/Session capacities 1 and 0.
	for _, partition := range []struct {
		name      string
		isDefault bool
		capacity  int
	}{{"A42Oracle", true, 1}, {"A42Zero", false, 0}} {
		writePlatformCachePartition(t, root, partition.name, partition.isDefault, partition.capacity)
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.CachePartitionFiles) != 2 {
		t.Fatalf("partition source discovery: %v", p.CachePartitionFiles)
	}
	org := storage.NewOrgState()
	if err := ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	partitions := org.Objects["PlatformCachePartition"].Records
	allocations := org.Objects["PlatformCachePartitionType"].Records
	if len(partitions) != 2 || len(allocations) != 4 {
		t.Fatalf("setup records: %d partitions, %d allocations", len(partitions), len(allocations))
	}
	for id, partition := range partitions {
		wantCapacity := int64(0)
		wantDefault := partition.Fields["DeveloperName"].String == "A42Oracle"
		if wantDefault {
			wantCapacity = 1
		}
		if partition.Fields["NamespacePrefix"].String != "" || partition.Fields["IsDefaultPartition"].Boolean != wantDefault {
			t.Fatalf("partition metadata: %#v", partition)
		}
		seen := map[string]bool{}
		for _, allocation := range allocations {
			if allocation.Fields["PlatformCachePartitionId"].ID != id {
				continue
			}
			seen[allocation.Fields["CacheType"].String] = true
			if allocation.Fields["AllocatedCapacity"].Integer != wantCapacity || allocation.Fields["AllocatedTrialCapacity"].Integer != 0 {
				t.Fatalf("allocation metadata: %#v", allocation)
			}
		}
		if !seen["Organization"] || !seen["Session"] || len(seen) != 2 {
			t.Fatalf("cache types: %v", seen)
		}
	}
	// Source setup survives the feature pass used by both CLI and test org
	// construction, and reloading must update the same records and relationships.
	storage.ApplyOrgShape(&org, []string{"PlatformCache"})
	if len(org.Objects["PlatformCachePartition"].Records) != 2 {
		t.Fatal("feature added a competing default partition")
	}
	before := org.Clone()
	if err := ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"PlatformCachePartition", "PlatformCachePartitionType"} {
		if !reflect.DeepEqual(org.Objects[name].Records, before.Objects[name].Records) {
			t.Fatalf("reloaded %s setup changed", name)
		}
	}
}

func TestProjectPlatformCacheRejectsSecondDefault(t *testing.T) {
	// M001/D001-D004 at API 62/67: a second local default is rejected,
	// with the original default and both cache scopes' routing unchanged.
	for _, reverse := range []bool{false, true} {
		root := t.TempDir()
		oracle := writePlatformCachePartition(t, root, "A42Oracle", true, 1)
		zero := writePlatformCachePartition(t, root, "A42Zero", false, 0)
		paths := []string{oracle, zero}
		if reverse {
			paths = []string{zero, oracle}
		}
		org := storage.NewOrgState()
		if err := ApplyProject(&org, project.Project{CachePartitionFiles: paths}); err != nil {
			t.Fatal(err)
		}
		before := org.Clone()
		writePlatformCachePartition(t, root, "A42Zero", true, 0)
		if err := ApplyProject(&org, project.Project{CachePartitionFiles: []string{zero}}); err == nil || err.Error() != "Only one default partition is allowed per organization" {
			t.Fatalf("second default: %v", err)
		}
		for _, name := range []string{"PlatformCachePartition", "PlatformCachePartitionType"} {
			if !reflect.DeepEqual(org.Objects[name].Records, before.Objects[name].Records) {
				t.Fatalf("rejected metadata changed %s", name)
			}
		}
		fresh := storage.NewOrgState()
		if err := ApplyProject(&fresh, project.Project{CachePartitionFiles: paths}); err == nil || err.Error() != "Only one default partition is allowed per organization" {
			t.Fatalf("two authored defaults: %v", err)
		}
		if len(fresh.Objects["PlatformCachePartition"].Records) != 0 || len(fresh.Objects["PlatformCachePartitionType"].Records) != 0 {
			t.Fatal("rejected source installed partial cache setup")
		}
	}
}

func TestProjectPlatformCacheDefaultReplacesImplicitFeatureFallback(t *testing.T) {
	// Preserve the existing local-only feature seed when loading the captured
	// project metadata; the synthetic seed is not a native authored default.
	org := storage.NewOrgState()
	storage.ApplyOrgShape(&org, []string{"PlatformCache"})
	path := writePlatformCachePartition(t, t.TempDir(), "A42Oracle", true, 1)
	if err := ApplyProject(&org, project.Project{CachePartitionFiles: []string{path}}); err != nil {
		t.Fatal(err)
	}
	defaults := 0
	for _, record := range org.Objects["PlatformCachePartition"].Records {
		if record.Fields["IsDefaultPartition"].Boolean {
			defaults++
			if record.Fields["DeveloperName"].String != "A42Oracle" {
				t.Fatalf("source default not selected: %#v", record)
			}
		}
	}
	if defaults != 1 {
		t.Fatalf("configured default count: %d", defaults)
	}
}

func TestProjectPlatformCachePartitionRejectsMalformedMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Bad.cachePartition-meta.xml")
	if err := os.WriteFile(path, []byte("<PlatformCachePartition>"), 0o600); err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	if err := ApplyProject(&org, project.Project{CachePartitionFiles: []string{path}}); err == nil {
		t.Fatal("malformed partition metadata was silently ignored")
	}
}
