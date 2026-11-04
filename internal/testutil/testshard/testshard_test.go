package testshard

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAssignmentIsDeterministicAndBalanced(t *testing.T) {
	names := []string{"TestA", "TestB", "TestC", "TestD", "TestE", "TestF"}
	durations := map[string]int64{
		"TestA": 600, "TestB": 500, "TestC": 300,
		"TestD": 200, "TestE": 200, "TestF": 100,
	}
	first := Assignment(names, durations, 2)
	second := Assignment([]string{"TestF", "TestE", "TestD", "TestC", "TestB", "TestA"}, durations, 2)
	for i := range first {
		if strings.Join(first[i], ",") != strings.Join(second[i], ",") {
			t.Fatalf("assignment depends on input order: %v vs %v", first, second)
		}
	}
	// LPT on these weights: shard0 = A+D+E (1000), shard1 = B+C+F (900).
	if strings.Join(first[0], ",") != "TestA,TestD,TestE" || strings.Join(first[1], ",") != "TestB,TestC,TestF" {
		t.Fatalf("unexpected balanced assignment: %v", first)
	}
	withNew := Assignment(append(names, "TestNew"), durations, 2)
	if len(withNew[0])+len(withNew[1]) != 7 {
		t.Fatalf("new test lost: %v", withNew)
	}
	if MedianDuration(nil) != 1 || MedianDuration(map[string]int64{"TestA": 0}) != 1 || MedianDuration(durations) != 200 {
		t.Fatalf("median = %d, %d, %d", MedianDuration(nil), MedianDuration(map[string]int64{"TestA": 0}), MedianDuration(durations))
	}
}

func TestRunPatternAndSpec(t *testing.T) {
	pattern, err := RunPattern([]string{"TestA", "TestB"}, "")
	if err != nil || pattern != "^(?:TestA|TestB)$" {
		t.Fatalf("pattern = %q, %v", pattern, err)
	}
	pattern, err = RunPattern([]string{"TestA", "TestB"}, "TestB/62\\.0")
	if err != nil || pattern != "^(?:TestB)$/62\\.0" {
		t.Fatalf("composed pattern = %q, %v", pattern, err)
	}
	pattern, err = RunPattern([]string{"TestA", "TestB"}, "^TestA$/x|^TestB$/y")
	if err != nil || pattern != "^(?:TestA)$/x|^(?:TestB)$/y" {
		t.Fatalf("alternation pattern = %q, %v", pattern, err)
	}
	if _, err := RunPattern([]string{"TestA"}, "("); err == nil {
		t.Fatal("invalid -run pattern was accepted")
	}
	for _, bad := range []string{"3/2", "1", "-1/2", "0/0", "0/5000", "a/b"} {
		if _, _, err := ParseSpec("X", bad); err == nil {
			t.Fatalf("spec %q was accepted", bad)
		}
	}
	index, count, err := ParseSpec("X", "0/1")
	if err != nil || index != 0 || count != 1 {
		t.Fatalf("ParseSpec(0/1) = %d/%d, %v", index, count, err)
	}
}

func TestLoadDurationTableRejectsBadTables(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	good := write("good.json", `{"version":1,"package":"p","source":"s","tests":[{"name":"TestA","durationMillis":5}]}`)
	table, err := LoadDurationTable(good, "p")
	if err != nil || table["TestA"] != 5 {
		t.Fatalf("good table: %v, %v", table, err)
	}
	for name, body := range map[string]string{
		"version.json":   `{"version":2,"package":"p","tests":[]}`,
		"package.json":   `{"version":1,"package":"q","tests":[]}`,
		"unknown.json":   `{"version":1,"package":"p","tests":[],"extra":1}`,
		"name.json":      `{"version":1,"package":"p","tests":[{"name":"testA","durationMillis":5}]}`,
		"negative.json":  `{"version":1,"package":"p","tests":[{"name":"TestA","durationMillis":-5}]}`,
		"duplicate.json": `{"version":1,"package":"p","tests":[{"name":"TestA","durationMillis":5},{"name":"TestA","durationMillis":6}]}`,
	} {
		if _, err := LoadDurationTable(write(name, body), "p"); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
	if _, err := LoadDurationTable(filepath.Join(dir, "missing.json"), "p"); err == nil {
		t.Error("missing table was accepted")
	}
}

func TestApplyWithoutSelectorIsNoop(t *testing.T) {
	t.Setenv("GLADE_TESTSHARD_UNIT_SHARD", "")
	if err := Apply("GLADE_TESTSHARD_UNIT_SHARD", "missing.json", "p"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GLADE_TESTSHARD_UNIT_SHARD", "2/1")
	if err := Apply("GLADE_TESTSHARD_UNIT_SHARD", "missing.json", "p"); err == nil {
		t.Fatal("malformed selector was accepted")
	}
}

// The package's own registry, sharded by its own helper.
func TestAssertOnThisPackage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shard_durations.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"package":"github.com/glade-sh/glade/internal/testutil/testshard","source":"unit","tests":[{"name":"TestAssertOnThisPackage","durationMillis":1}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	Assert(t, "GLADE_TESTSHARD_UNIT_SHARD", path, "github.com/glade-sh/glade/internal/testutil/testshard", 3)
}

// A shard selector must not leak into child processes: a re-exec'd test binary
// with another working directory would re-apply it and fail on the relative
// duration table.
func TestApplyClearsSelectorForChildren(t *testing.T) {
	const env = "GLADE_TESTSHARD_UNIT_SHARD"
	path := filepath.Join(t.TempDir(), "shard_durations.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"package":"p","source":"unit","tests":[{"name":"TestAssertOnThisPackage","durationMillis":1}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	runFlag := flag.Lookup("test.run")
	original := runFlag.Value.String()
	t.Cleanup(func() { _ = runFlag.Value.Set(original) })
	t.Setenv(env, "0/1")
	if err := Apply(env, path, "p"); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv(env); got != "" {
		t.Fatalf("selector %s=%q still set after Apply; child processes would inherit it", env, got)
	}
}
