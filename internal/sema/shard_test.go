package sema

import (
	"fmt"
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/testutil/testshard"
)

// GLADE_SEMA_SHARD=i/n runs shard i of n of this package's top-level tests (see
// internal/testutil/testshard). TestShardPartitionMatchesUnshardedTestSet
// proves the shards together are exactly the unsharded set.
const (
	shardSelectorEnv       = "GLADE_SEMA_SHARD"
	shardDurationTablePath = "testdata/shard_durations.json"
	shardTestPackage       = "github.com/glade-sh/glade/internal/sema"
)

func TestMain(m *testing.M) {
	if err := testshard.Apply(shardSelectorEnv, shardDurationTablePath, shardTestPackage); err != nil {
		fmt.Fprintf(os.Stderr, "sema: %v\n", err)
		os.Exit(2)
	}
	os.Exit(m.Run())
}

func TestShardPartitionMatchesUnshardedTestSet(t *testing.T) {
	testshard.Assert(t, shardSelectorEnv, shardDurationTablePath, shardTestPackage, 1000)
}
