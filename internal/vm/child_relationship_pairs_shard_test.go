package vm

import "testing"

func TestChildRelationshipPairsShardCoverage(t *testing.T) {
	for _, count := range []int{0, 1, 2, 7, 20} {
		for _, shards := range []int{1, 2, 3, 4, 8, 32} {
			seen := map[[2]int]int{}
			for shard := 0; shard < shards; shard++ {
				for _, pair := range childRelPairsShard(count, shard, shards) {
					if pair[0] < 0 || pair[0] >= pair[1] || pair[1] >= count {
						t.Fatalf("count=%d shards=%d: invalid pair %v", count, shards, pair)
					}
					seen[pair]++
				}
			}
			for i := 0; i < count; i++ {
				for j := i + 1; j < count; j++ {
					if seen[[2]int{i, j}] != 1 {
						t.Errorf("count=%d shards=%d pair=(%d,%d): got %d occurrences", count, shards, i, j, seen[[2]int{i, j}])
					}
				}
			}
			if len(seen) != count*(count-1)/2 {
				t.Errorf("count=%d shards=%d: got %d pairs", count, shards, len(seen))
			}
		}
	}
}
