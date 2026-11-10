package main

import (
	"fmt"
	"reflect"
	"testing"
)

func TestZeroHistoryBalancesShardCounts(t *testing.T) {
	names := make([]string, 4000)
	durations := make(map[string]int64)
	for i := range names {
		names[i] = fmt.Sprintf("TestSynthetic%04d", i)
		durations[names[i]] = 0
	}
	for i := 0; i < 4; i++ {
		durations[names[i]] = 1000
	}
	history := validHistory(names, durations)
	got, diagnostic, err := buildPlan(names, 4, history)
	if err != nil || diagnostic != "" {
		t.Fatalf("plan: %v %s", err, diagnostic)
	}
	assertValidPlan(t, got, names)
	for _, shard := range got.Shards {
		if len(shard.Tests) < 990 || len(shard.Tests) > 1010 {
			t.Fatalf("unbalanced shard count: %d", len(shard.Tests))
		}
	}
	again, _, err := buildPlan(names, 4, history)
	if err != nil || !reflect.DeepEqual(got, again) {
		t.Fatal("plan is not deterministic")
	}
}

func TestZeroHistoryFloorOverflowFallsBack(t *testing.T) {
	names := []string{"TestLarge", "TestZero"}
	durations := map[string]int64{"TestLarge": 9223372036854775807, "TestZero": 0}
	got, diagnostic, err := buildPlan(names, 2, validHistory(names, durations))
	if err != nil || got.HistoryUsed || diagnostic == "" {
		t.Fatalf("expected overflow fallback: plan=%+v diagnostic=%q error=%v", got, diagnostic, err)
	}
	assertValidPlan(t, got, names)
}
