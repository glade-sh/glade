package vm_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/testutil/testshard"
)

// GLADE_VM_SHARD=i/n runs shard i of n of this package's top-level tests (see
// internal/testutil/testshard). The 4,000-plus vm tests run serially, so CI
// lanes split them across processes; TestShardPartitionMatchesUnshardedTestSet
// proves the shards together are exactly the unsharded set.
const (
	shardSelectorEnv       = "GLADE_VM_SHARD"
	shardDurationTablePath = "testdata/shard_durations.json"
	shardTestPackage       = "github.com/glade-sh/glade/internal/vm"
)

func TestMain(m *testing.M) {
	if err := testshard.Apply(shardSelectorEnv, shardDurationTablePath, shardTestPackage); err != nil {
		fmt.Fprintf(os.Stderr, "vm: %v\n", err)
		os.Exit(2)
	}
	// Measured on shard 3/8: releasing the decoded catalog cuts peak RSS by
	// about a quarter for about 3% more instructions. The apextest binary does
	// not opt in: measured there, the release added work and saved no memory.
	// gladecli and server drive the same runner and stay off until measured.
	storage.SetStandardCatalogReleaseEnabled(true)
	os.Exit(m.Run())
}

func TestShardPartitionMatchesUnshardedTestSet(t *testing.T) {
	testshard.Assert(t, shardSelectorEnv, shardDurationTablePath, shardTestPackage, 3000)
}
