package main

import (
	"bytes"
	"encoding/json"
	"math"
	"testing"
)

func TestNewTestsReceiveEstimatesAndSpread(t *testing.T) {
	for _, tc := range []struct {
		name      string
		durations map[string]int64
	}{
		{"unequal", map[string]int64{"TestKnownA": 10, "TestKnownB": 8}},
		{"zero", map[string]int64{"TestKnownA": 0, "TestKnownB": 0}},
		{"empty", map[string]int64{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			known := make([]string, 0, len(tc.durations))
			for n := range tc.durations {
				known = append(known, n)
			}
			names := append(append([]string{}, known...), "TestNewA", "TestNewB", "TestNewC", "TestNewD", "TestNewE", "TestNewF")
			history := validHistory(known, tc.durations)
			got, diagnostic, err := buildPlan(names, 2, history)
			if err != nil || diagnostic != "" || !got.HistoryUsed {
				t.Fatalf("plan=%+v diagnostic=%q err=%v", got, diagnostic, err)
			}
			assertValidPlan(t, got, names)
			for _, shard := range got.Shards {
				newCount := 0
				for _, name := range shard.Tests {
					if len(name) >= 7 && name[:7] == "TestNew" {
						newCount++
					}
				}
				if newCount == 0 {
					t.Fatalf("new tests did not spread: %+v", got.Shards)
				}
			}
			var total int64
			for _, shard := range got.Shards {
				total += shard.EstimatedDurationMillis
			}
			var knownTotal int64
			for _, n := range tc.durations {
				knownTotal += n
			}
			if total <= knownTotal {
				t.Fatalf("new tests received zero total estimate: %d", total-knownTotal)
			}
			reversed := append([]string(nil), names...)
			for i, j := 0, len(reversed)-1; i < j; i, j = i+1, j-1 {
				reversed[i], reversed[j] = reversed[j], reversed[i]
			}
			other, _, err := buildPlan(reversed, 2, history)
			if err != nil {
				t.Fatal(err)
			}
			a, _ := json.Marshal(got)
			b, _ := json.Marshal(other)
			if !bytes.Equal(a, b) {
				t.Fatal("discovery order changed plan")
			}
		})
	}
}

func TestNewTestEstimateDoesNotOverflow(t *testing.T) {
	names := []string{"TestKnown", "TestNew"}
	got, _, err := buildPlan(names, 1, validHistory([]string{"TestKnown"}, map[string]int64{"TestKnown": math.MaxInt64}))
	if err != nil {
		t.Fatal(err)
	}
	if got.Shards[0].EstimatedDurationMillis < 0 {
		t.Fatalf("overflowed estimate: %+v", got)
	}
}
