package vm_test

import (
	"fmt"
	"os"
	"testing"

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
	os.Exit(m.Run())
}

func TestShardPartitionMatchesUnshardedTestSet(t *testing.T) {
	testshard.Assert(t, shardSelectorEnv, shardDurationTablePath, shardTestPackage, 3000)
}
