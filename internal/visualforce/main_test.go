package visualforce_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/testutil/testshard"
)

// GLADE_VISUALFORCE_SHARD=i/n runs shard i of n of this package's top-level
// tests (see internal/testutil/testshard). The family conformance tests run
// serially and dominate the package, so CI lanes split them across processes;
// TestShardPartitionMatchesUnshardedTestSet proves the shards together are
// exactly the unsharded set.
const (
	shardSelectorEnv       = "GLADE_VISUALFORCE_SHARD"
	shardDurationTablePath = "testdata/shard_durations.json"
	shardTestPackage       = "github.com/glade-sh/glade/internal/visualforce"
)

func TestMain(m *testing.M) {
	if err := testshard.Apply(shardSelectorEnv, shardDurationTablePath, shardTestPackage); err != nil {
		fmt.Fprintf(os.Stderr, "visualforce: %v\n", err)
		os.Exit(2)
	}
	restoreDiskCache := apextest.DisableDiskCacheForTesting()
	code := m.Run()
	restoreDiskCache()
	os.Exit(code)
}

func TestShardPartitionMatchesUnshardedTestSet(t *testing.T) {
	testshard.Assert(t, shardSelectorEnv, shardDurationTablePath, shardTestPackage, 200)
}
