package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A single test whose raced budget exceeds the balanced per-shard budget
// gets a shard of its own: the isolated tests own the trailing shards in
// sorted order, one each, and the remaining tests balance across the rest.
func TestIsolatePinsGiantTestsToTrailingShards(t *testing.T) {
	names := []string{"TestAlpha", "TestBeta", "TestDelta", "TestGamma"}
	history := `{"version":1,"package":"github.com/glade-sh/glade/internal/apextest","complete":true,"tests":[
{"name":"TestAlpha","durationMillis":100},
{"name":"TestBeta","durationMillis":200},
{"name":"TestDelta","durationMillis":300},
{"name":"TestGamma","durationMillis":9000000}
]}`
	got, diagnostic, err := buildPlanForPackage(apexTestPackage, names, 3, []byte(history), []string{"TestGamma"})
	if err != nil {
		t.Fatalf("plan failed: %v diagnostic=%s", err, diagnostic)
	}
	if len(got.Shards) != 3 {
		t.Fatalf("plan has %d shards, want 3", len(got.Shards))
	}
	last := got.Shards[2]
	if len(last.Tests) != 1 || last.Tests[0] != "TestGamma" {
		t.Fatalf("isolated shard = %#v, want only TestGamma", last)
	}
	if last.EstimatedDurationMillis != 9000000 {
		t.Fatalf("isolated shard estimate = %d, want the giant's 9000000", last.EstimatedDurationMillis)
	}
	seen := map[string]bool{}
	for _, shard := range got.Shards[:2] {
		for _, name := range shard.Tests {
			if name == "TestGamma" {
				t.Fatalf("the giant test also landed in balanced shard %d", shard.Index)
			}
			seen[name] = true
		}
	}
	for _, name := range names[:3] {
		if !seen[name] {
			t.Fatalf("test %s missing from the balanced shards", name)
		}
	}
	if err := validatePlan(got, names); err != nil {
		t.Fatalf("plan validation: %v", err)
	}
}

// Two isolated tests occupy the last two shards in sorted order regardless
// of the order they were requested in.
func TestIsolateOrdersMultipleGiants(t *testing.T) {
	names := []string{"TestA", "TestB", "TestC", "TestD", "TestE"}
	history := validHistory(names, map[string]int64{"TestC": 8000000, "TestE": 9000000})
	got, _, err := buildPlanForPackage(apexTestPackage, names, 4, []byte(history), []string{"TestE", "TestC"})
	if err != nil {
		t.Fatalf("plan failed: %v", err)
	}
	if got.Shards[2].Tests[0] != "TestC" || got.Shards[3].Tests[0] != "TestE" {
		t.Fatalf("isolated shards = %#v %#v, want TestC then TestE", got.Shards[2].Tests, got.Shards[3].Tests)
	}
}

// An isolated name that is not in the discovery, a duplicate request, or a
// count that leaves no balanced shard must fail the plan loudly.
func TestIsolateRejectsUnknownDuplicateAndCrowdedRequests(t *testing.T) {
	names := []string{"TestA", "TestB", "TestC"}
	history := validHistory(names, nil)
	if _, _, err := buildPlanForPackage(apexTestPackage, names, 3, []byte(history), []string{"TestMissing"}); err == nil {
		t.Fatal("an isolated test outside the discovery must fail the plan")
	}
	if _, _, err := buildPlanForPackage(apexTestPackage, names, 3, []byte(history), []string{"TestA", "TestA"}); err == nil {
		t.Fatal("a duplicate isolated request must fail the plan")
	}
	if _, _, err := buildPlanForPackage(apexTestPackage, names, 3, []byte(history), []string{"TestA", "TestB", "TestC"}); err == nil {
		t.Fatal("isolating every shard must fail the plan")
	}
	if _, err := readIsolatedTests(""); err != nil {
		t.Fatalf("an empty isolation list must be allowed: %v", err)
	}
	if _, err := readIsolatedTests("TestA,,TestB"); err == nil {
		t.Fatal("a blank isolated name must fail")
	}
}

// The command line accepts the comma-separated isolation list the lane
// passes from the plan's isolated tests.
func TestIsolateCommandLinePinsTrailingShard(t *testing.T) {
	historyPath := filepath.Join(t.TempDir(), "history.json")
	if err := os.WriteFile(historyPath, validHistory([]string{"TestA", "TestB", "TestC"}, map[string]int64{"TestC": 9000000}), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	rc := run([]string{
		"--package", apexTestPackage,
		"--shards", "3",
		"--tests", "", // read the discovery from stdin
		"--history", historyPath,
		"--isolate", "TestC",
	}, strings.NewReader("TestA\nTestB\nTestC\n"), &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("isolate plan rc=%d stderr=%s", rc, stderr.String())
	}
	var got plan
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("plan JSON: %v\n%s", err, stdout.String())
	}
	if len(got.Shards) != 3 || len(got.Shards[2].Tests) != 1 || got.Shards[2].Tests[0] != "TestC" {
		t.Fatalf("plan shards = %#v", got.Shards)
	}
	if !got.HistoryUsed {
		t.Fatal("plan must record the history it used")
	}
	if err := validatePlan(got, []string{"TestA", "TestB", "TestC"}); err != nil {
		t.Fatalf("plan validation: %v", err)
	}
}
