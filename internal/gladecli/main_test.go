package gladecli

import (
	"fmt"
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/testutil/testshard"
)

// GLADE_GLADECLI_SHARD=i/n runs shard i of n of this package's top-level tests
// (see internal/testutil/testshard). TestShardPartitionMatchesUnshardedTestSet
// proves the shards together are exactly the unsharded set.
const (
	shardSelectorEnv       = "GLADE_GLADECLI_SHARD"
	shardDurationTablePath = "testdata/shard_durations.json"
	shardTestPackage       = "github.com/glade-sh/glade/internal/gladecli"
)

func TestMain(m *testing.M) {
	if err := testshard.Apply(shardSelectorEnv, shardDurationTablePath, shardTestPackage); err != nil {
		fmt.Fprintf(os.Stderr, "gladecli: %v\n", err)
		os.Exit(2)
	}
	cleanupPrivateTemp := preparePrivateTestTemp()
	restoreDiskCache := apextest.DisableDiskCacheForTesting()
	code := m.Run()
	restoreDiskCache()
	if err := cleanupPrivateTemp(); err != nil {
		_, _ = os.Stderr.WriteString("clean private test temp: " + err.Error() + "\n")
		code = 1
	}
	os.Exit(code)
}

func TestShardPartitionMatchesUnshardedTestSet(t *testing.T) {
	testshard.Assert(t, shardSelectorEnv, shardDurationTablePath, shardTestPackage, 400)
}
