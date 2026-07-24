package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCIPackageManifestLoadsExactLivePackageSet(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	manifestPath := filepath.Join(repoRoot, "scripts", "ci-package-lanes.json")
	cmd := exec.Command("go", "list", "./...")
	cmd.Dir = repoRoot
	packages, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	packagesPath := filepath.Join(t.TempDir(), "packages.txt")
	if err := os.WriteFile(packagesPath, packages, 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	rc := run([]string{"--package-manifest", manifestPath, "--packages", packagesPath}, strings.NewReader(""), &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("manifest load rc=%d stderr=%s", rc, stderr.String())
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	wantCount := len(strings.Fields(string(packages)))
	if len(lines) != wantCount {
		t.Fatalf("owned package rows = %d, want live package count %d", len(lines), wantCount)
	}
	for _, lane := range []string{"apextest", "gladecli", "sema", "server-and-playground", "repoguard", "remaining-go"} {
		if !strings.Contains(stdout.String(), lane+"\t") {
			t.Errorf("manifest output missing lane %q", lane)
		}
	}
	if err := os.WriteFile(packagesPath, append(packages, []byte("github.com/glade-sh/glade/internal/newpackage\n")...), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if rc := run([]string{"--package-manifest", manifestPath, "--packages", packagesPath}, strings.NewReader(""), &stdout, &stderr); rc == 0 {
		t.Fatal("new current package was silently accepted without manifest ownership")
	}
}

func TestCIPackageManifestRejectsUncertainOwnership(t *testing.T) {
	packagesPath := filepath.Join(t.TempDir(), "packages.txt")
	if err := os.WriteFile(packagesPath, []byte("example.test/a\nexample.test/b\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"malformed JSON":     `{`,
		"duplicate JSON key": `{"version":1,"version":1,"lanes":{}}`,
		"unknown field":      `{"version":1,"lanes":{"apextest":["example.test/a"],"gladecli":["example.test/b"],"sema":["example.test/c"],"server-and-playground":["example.test/d"],"repoguard":["example.test/e"],"remaining-go":["example.test/f"]},"extra":true}`,
		"duplicate package":  `{"version":1,"lanes":{"apextest":["example.test/a"],"gladecli":["example.test/a"],"sema":["example.test/b"],"server-and-playground":["example.test/c"],"repoguard":["example.test/d"],"remaining-go":["example.test/e"]}}`,
		"unknown lane":       `{"version":1,"lanes":{"apextest":["example.test/a"],"gladecli":["example.test/b"],"sema":["example.test/c"],"server-and-playground":["example.test/d"],"repoguard":["example.test/e"],"remaining-go":["example.test/f"],"surprise":["example.test/g"]}}`,
		"empty ownership":    `{"version":1,"lanes":{"apextest":[],"gladecli":["example.test/a"],"sema":["example.test/b"],"server-and-playground":["example.test/c"],"repoguard":["example.test/d"],"remaining-go":["example.test/e"]}}`,
		"package mismatch":   `{"version":1,"lanes":{"apextest":["example.test/a"],"gladecli":["example.test/b"],"sema":["example.test/c"],"server-and-playground":["example.test/d"],"repoguard":["example.test/e"],"remaining-go":["example.test/f"]}}`,
	}
	for name, contents := range cases {
		t.Run(name, func(t *testing.T) {
			manifestPath := filepath.Join(t.TempDir(), "manifest.json")
			if err := os.WriteFile(manifestPath, []byte(contents), 0o600); err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			if rc := run([]string{"--package-manifest", manifestPath, "--packages", packagesPath}, strings.NewReader(""), &stdout, &stderr); rc == 0 {
				t.Fatalf("invalid manifest accepted: %s", stdout.String())
			}
			if stdout.Len() != 0 || stderr.Len() == 0 {
				t.Fatalf("stdout/stderr = %q/%q", stdout.String(), stderr.String())
			}
		})
	}
}

func validHistory(names []string, durations map[string]int64) []byte {
	tests := make([]historyTest, 0, len(names))
	for _, name := range names {
		duration := durations[name]
		tests = append(tests, historyTest{Name: name, DurationMillis: &duration})
	}
	b, err := json.Marshal(historyFile{
		Version:  1,
		Package:  apexTestPackage,
		Complete: true,
		Tests:    tests,
	})
	if err != nil {
		panic(err)
	}
	return b
}

func assertValidPlan(t *testing.T, plan plan, names []string) {
	t.Helper()
	seen := make(map[string]bool, len(names))
	for shardIndex, shard := range plan.Shards {
		if shard.Index != shardIndex {
			t.Fatalf("shard index = %d, want %d", shard.Index, shardIndex)
		}
		if len(shard.Tests) == 0 {
			t.Fatalf("shard %d is empty", shardIndex)
		}
		for i, name := range shard.Tests {
			if i > 0 && shard.Tests[i-1] >= name {
				t.Fatalf("shard %d tests are not lexical: %v", shardIndex, shard.Tests)
			}
			if seen[name] {
				t.Fatalf("duplicate test in plan: %s", name)
			}
			seen[name] = true
		}
	}
	if len(seen) != len(names) {
		t.Fatalf("plan union has %d tests, want %d", len(seen), len(names))
	}
	for _, name := range names {
		if !seen[name] {
			t.Fatalf("plan omitted %s", name)
		}
	}
}

func TestBuildPlanUsesDeterministicLPTAndShardIndexTieBreak(t *testing.T) {
	names := []string{"TestDelta", "TestAlpha", "TestCharlie", "TestBravo"}
	history := validHistory(names, map[string]int64{
		"TestAlpha": 10, "TestBravo": 10, "TestCharlie": 5, "TestDelta": 5,
	})

	got, diagnostic, err := buildPlan(names, 2, history)
	if err != nil {
		t.Fatal(err)
	}
	if diagnostic != "" {
		t.Fatalf("unexpected diagnostic: %s", diagnostic)
	}
	assertValidPlan(t, got, names)
	if !got.HistoryUsed {
		t.Fatal("valid complete history was not used")
	}
	want := [][]string{{"TestAlpha", "TestCharlie"}, {"TestBravo", "TestDelta"}}
	for i := range want {
		if !reflect.DeepEqual(got.Shards[i].Tests, want[i]) {
			t.Fatalf("shard %d tests = %v, want %v", i, got.Shards[i].Tests, want[i])
		}
		if got.Shards[i].EstimatedDurationMillis != 15 {
			t.Fatalf("shard %d estimate = %d, want 15", i, got.Shards[i].EstimatedDurationMillis)
		}
	}
}

func TestBuildPlanUsesRequestedPackage(t *testing.T) {
	const semaPackage = "github.com/glade-sh/glade/internal/sema"
	names := []string{"TestAlpha", "TestBravo"}
	history := validHistory(names, map[string]int64{"TestAlpha": 2, "TestBravo": 1})
	history = bytes.Replace(history, []byte(apexTestPackage), []byte(semaPackage), 1)
	historyPath := filepath.Join(t.TempDir(), "history.json")
	if err := os.WriteFile(historyPath, history, 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	rc := run([]string{"--package", semaPackage, "--shards", "2", "--history", historyPath}, strings.NewReader("TestBravo\nTestAlpha\n"), &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("requested package plan rc=%d stderr=%s", rc, stderr.String())
	}
	var got plan
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Package != semaPackage || !got.HistoryUsed {
		t.Fatalf("package/historyUsed = %q/%v, want %q/true", got.Package, got.HistoryUsed, semaPackage)
	}
}

func TestRequestedPackageAndHistoryMustBeExact(t *testing.T) {
	for _, packageName := range []string{"", "./internal/sema", "github.com/glade-sh/glade/internal/...", "github.com/glade-sh/glade/internal/sema/", "github.com/glade-sh/glade/internal/.", "github.com/glade-sh/glade/internal/..", "example.com/foo.", "example.com/foo./bar"} {
		var stdout, stderr bytes.Buffer
		if rc := run([]string{"--package", packageName}, strings.NewReader("TestA\nTestB\n"), &stdout, &stderr); rc == 0 {
			t.Fatalf("invalid exact package %q accepted: %s", packageName, stdout.String())
		}
	}
	const semaPackage = "github.com/glade-sh/glade/internal/sema"
	history := []byte(`{"version":1,"version":1,"package":"` + semaPackage + `","complete":true,"tests":[{"name":"TestA","durationMillis":1},{"name":"TestB","durationMillis":1}]}`)
	got, diagnostic, err := buildPlanForPackage(semaPackage, []string{"TestA", "TestB"}, 2, history)
	if err != nil {
		t.Fatal(err)
	}
	if diagnostic == "" || got.HistoryUsed {
		t.Fatalf("duplicate-key history accepted: diagnostic=%q plan=%+v", diagnostic, got)
	}
}

func TestBuildPlanIsByteIdenticalAcrossDiscoveryOrder(t *testing.T) {
	names := []string{"TestD", "TestA", "TestC", "TestB"}
	history := validHistory(names, map[string]int64{"TestA": 7, "TestB": 6, "TestC": 5, "TestD": 4})

	first, _, err := buildPlan(names, 2, history)
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := buildPlan([]string{"TestB", "TestD", "TestA", "TestC"}, 2, history)
	if err != nil {
		t.Fatal(err)
	}
	firstJSON, _ := json.Marshal(first)
	secondJSON, _ := json.Marshal(second)
	if !bytes.Equal(firstJSON, secondJSON) {
		t.Fatalf("plans differ by discovery order:\n%s\n%s", firstJSON, secondJSON)
	}
}

func TestFallbackIsDeterministicAndBalances279Tests(t *testing.T) {
	names := make([]string, 279)
	for i := range names {
		names[i] = fmt.Sprintf("TestCase%03d", i)
	}
	got, diagnostic, err := buildPlan(names, 2, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(diagnostic, "history") || got.HistoryUsed {
		t.Fatalf("fallback diagnostic/historyUsed = %q/%v", diagnostic, got.HistoryUsed)
	}
	assertValidPlan(t, got, names)
	if len(got.Shards[0].Tests) != 140 || len(got.Shards[1].Tests) != 139 {
		t.Fatalf("fallback sizes = %d/%d, want 140/139", len(got.Shards[0].Tests), len(got.Shards[1].Tests))
	}
	shuffled := append([]string(nil), names...)
	for left, right := 0, len(shuffled)-1; left < right; left, right = left+1, right-1 {
		shuffled[left], shuffled[right] = shuffled[right], shuffled[left]
	}
	again, _, err := buildPlan(shuffled, 2, nil)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(got)
	b, _ := json.Marshal(again)
	if !bytes.Equal(a, b) {
		t.Fatal("fallback output changed with discovery order")
	}
}

func TestHistoryRejectionFallsBackWholesale(t *testing.T) {
	names := []string{"TestA", "TestB", "TestC"}
	valid := string(validHistory(names, map[string]int64{"TestA": 0, "TestB": 2, "TestC": 3}))
	cases := map[string]string{
		"malformed":         `{`,
		"negative":          strings.Replace(valid, `"durationMillis":2`, `"durationMillis":-2`, 1),
		"duplicate":         strings.Replace(valid, `"name":"TestC"`, `"name":"TestA"`, 1),
		"missing":           strings.Replace(valid, `,{"name":"TestC","durationMillis":3}`, ``, 1),
		"extra":             strings.Replace(valid, `]}`, `,{"name":"TestExtra","durationMillis":1}]}`, 1),
		"stale":             strings.Replace(valid, `"name":"TestC"`, `"name":"TestStale"`, 1),
		"wrong version":     strings.Replace(valid, `"version":1`, `"version":2`, 1),
		"wrong package":     strings.Replace(valid, apexTestPackage, `example.invalid/apextest`, 1),
		"incomplete":        strings.Replace(valid, `"complete":true`, `"complete":false`, 1),
		"unknown field":     strings.Replace(valid, `"version":1`, `"version":1,"failed":true`, 1),
		"invalid test name": strings.Replace(valid, `"name":"TestC"`, `"name":"BenchmarkC"`, 1),
		"missing duration":  strings.Replace(valid, `,"durationMillis":2`, ``, 1),
		"duration overflow": `{"version":1,"package":"` + apexTestPackage + `","complete":true,"tests":[{"name":"TestA","durationMillis":9223372036854775807},{"name":"TestB","durationMillis":9223372036854775807},{"name":"TestC","durationMillis":0}]}`,
	}
	for name, history := range cases {
		t.Run(name, func(t *testing.T) {
			got, diagnostic, err := buildPlan(names, 2, []byte(history))
			if err != nil {
				t.Fatalf("history rejection must use fallback, got hard error: %v", err)
			}
			if diagnostic == "" || got.HistoryUsed {
				t.Fatalf("diagnostic/historyUsed = %q/%v", diagnostic, got.HistoryUsed)
			}
			assertValidPlan(t, got, names)
			for _, shard := range got.Shards {
				if shard.EstimatedDurationMillis != 0 {
					t.Fatalf("fallback retained partial history weight: %+v", shard)
				}
			}
		})
	}
}

func TestZeroDurationHistoryIsValid(t *testing.T) {
	names := []string{"TestA", "TestB"}
	got, diagnostic, err := buildPlan(names, 2, validHistory(names, map[string]int64{}))
	if err != nil || diagnostic != "" || !got.HistoryUsed {
		t.Fatalf("zero history got plan=%+v diagnostic=%q err=%v", got, diagnostic, err)
	}
}

func TestHardFailureInputs(t *testing.T) {
	cases := []struct {
		name   string
		tests  []string
		shards int
	}{
		{"empty", nil, 2},
		{"invalid name", []string{"BenchmarkA", "TestB"}, 2},
		{"whitespace", []string{"Test A", "TestB"}, 2},
		{"duplicate", []string{"TestA", "TestA"}, 2},
		{"zero shards", []string{"TestA"}, 0},
		{"too many shards", []string{"TestA"}, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := buildPlan(tc.tests, tc.shards, nil); err == nil {
				t.Fatal("expected hard failure")
			}
		})
	}
}

func TestRegexIsAnchoredQuotedAndStable(t *testing.T) {
	if got, want := makeRegex([]string{"TestA+B", "TestA.B"}), `^(?:TestA\+B|TestA\.B)$`; got != want {
		t.Fatalf("regex = %q, want %q", got, want)
	}
}

func TestValidatePlanRejectsNonCanonicalTestOrder(t *testing.T) {
	names := []string{"TestA", "TestB"}
	bad := plan{Version: 1, Package: apexTestPackage, Shards: []shardPlan{{
		Index: 0, Tests: []string{"TestB", "TestA"}, Regex: makeRegex([]string{"TestB", "TestA"}),
	}}}
	if err := validatePlan(bad, names); err == nil {
		t.Fatal("expected non-lexical shard order to fail validation")
	}
}

func TestRunReadsStdinOrTestsFileAndSelectsIndex(t *testing.T) {
	testsPath := filepath.Join(t.TempDir(), "tests.txt")
	if err := os.WriteFile(testsPath, []byte("TestA\nTestB\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		args  []string
		input string
	}{
		{"stdin", []string{"--shards", "2", "--index", "1"}, "TestA\nTestB\n"},
		{"tests file", []string{"--tests", testsPath, "--shards", "2", "--index", "1"}, "ignored malformed input\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			rc := run(tc.args, strings.NewReader(tc.input), &stdout, &stderr)
			if rc != 0 {
				t.Fatalf("run rc=%d stderr=%s", rc, stderr.String())
			}
			var shard shardPlan
			if err := json.Unmarshal(stdout.Bytes(), &shard); err != nil {
				t.Fatalf("decode selected shard: %v; output=%s", err, stdout.String())
			}
			if shard.Index != 1 || len(shard.Tests) != 1 {
				t.Fatalf("selected shard = %+v", shard)
			}
		})
	}
}

func TestRunRejectsInvalidIndexAndMalformedDiscovery(t *testing.T) {
	for _, tc := range []struct {
		name  string
		args  []string
		input string
	}{
		{"negative index", []string{"--index", "-1"}, "TestA\nTestB\n"},
		{"high index", []string{"--index", "2"}, "TestA\nTestB\n"},
		{"blank line", nil, "TestA\n\nTestB\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if rc := run(tc.args, strings.NewReader(tc.input), &stdout, &stderr); rc == 0 {
				t.Fatalf("expected failure; output=%s", stdout.String())
			}
			if stdout.Len() != 0 || stderr.Len() == 0 {
				t.Fatalf("stdout/stderr = %q/%q", stdout.String(), stderr.String())
			}
		})
	}
}
