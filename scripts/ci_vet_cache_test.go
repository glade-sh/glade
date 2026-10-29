package scripts

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Use an import-free owned module so these guards exercise Go's real vet cache
// without recompiling the standard library or depending on the repository cache.
type vetCacheFixture struct {
	goCommand string
	wrapper   string
	cache     string
	first     string
	second    string
}

func newVetCacheFixture(t *testing.T) vetCacheFixture {
	t.Helper()
	wrapper, err := filepath.Abs("ci-go-test.sh")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	f := vetCacheFixture{
		goCommand: realGoCommand(t), wrapper: wrapper,
		cache: filepath.Join(root, "shared-cache"),
		first: filepath.Join(root, "first-checkout"), second: filepath.Join(root, "second-checkout"),
	}
	for _, dir := range []string{f.first, f.second} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.test/vetcache\n\ngo 1.26\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		f.source(t, dir, "func Sum(a, b int) int { return a + b }\n")
	}
	return f
}

func (f vetCacheFixture) source(t *testing.T, dir, source string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "fixture.go"), []byte("package vetcache\n"+source), 0o600); err != nil {
		t.Fatal(err)
	}
}

func (f vetCacheFixture) run(t *testing.T, dir string, trimmed, trace bool) (string, error) {
	t.Helper()
	cmd := exec.Command(f.goCommand, "vet", "./...")
	if trimmed {
		cmd = exec.Command("bash", f.wrapper, "local-release-lane", "vet")
	}
	cmd.Dir = dir
	flags := "-p=1"
	if trace {
		flags += " -x"
	}
	cmd.Env = append(os.Environ(),
		"PATH="+filepath.Dir(f.goCommand)+string(os.PathListSeparator)+os.Getenv("PATH"),
		"GOENV=off", "GOWORK=off", "GOTOOLCHAIN=local", "GOMAXPROCS=1",
		"GOCACHE="+f.cache, "GOFLAGS="+flags, "LOCAL_GO_TEST_JOBS=1",
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if trace {
		// The lane forwards successful captured output to stdout.
		cmd.Stdout = &stderr
	}
	err := cmd.Run()
	return stderr.String(), err
}

func vetToolExecutions(trace string) int {
	return len(regexp.MustCompile(`(?m)^.*[/\\]vet(?:\.exe)? [^\n]*vet\.cfg\r?$`).FindAllString(trace, -1))
}

func TestCIVetCacheAcrossCheckoutPaths(t *testing.T) {
	for _, trimmed := range []bool{false, true} {
		name := "untrimmed-control"
		if trimmed {
			name = "release-vet-lane"
		}
		t.Run(name, func(t *testing.T) {
			f := newVetCacheFixture(t)
			for i, dir := range []string{f.first, f.second} {
				trace, err := f.run(t, dir, trimmed, true)
				want := 1
				if trimmed && i == 1 {
					want = 0
				}
				if err != nil || vetToolExecutions(trace) != want {
					t.Fatalf("checkout %d: vet executions = %d, want %d; %v\n%s", i, vetToolExecutions(trace), want, err, trace)
				}
			}
			// A real source edit must invalidate even the warmed cross-path result.
			f.source(t, f.second, "func Sum(a, b int) int { return a - b }\n")
			trace, err := f.run(t, f.second, trimmed, true)
			if err != nil || vetToolExecutions(trace) != 1 {
				t.Fatalf("edited checkout reused stale vet: %v\n%s", err, trace)
			}
		})
	}
}

func TestCIVetFailureRerunsWithoutTrimpath(t *testing.T) {
	dir := t.TempDir()
	goScript := `#!/usr/bin/env bash
printf '%s GOCACHE=%s GOFLAGS=%s\n' "$*" "$GOCACHE" "$GOFLAGS" >> "$VET_CALLS"
if [[ "$*" == 'vet -trimpath ./...' ]]; then
  echo stale-checkout-stdout
  echo stale-checkout-stderr >&2
  exit 17
fi
if [[ "$*" == 'vet -trimpath=false ./...' ]]; then
  echo current-checkout-stdout
  echo current-checkout-stderr >&2
  exit 23
fi
exit 91
`
	writeApexFixtureExecutable(t, filepath.Join(dir, "go"), goScript)
	cmd := exec.Command("bash", "ci-go-test.sh", "local-release-lane", "vet")
	cache := filepath.Join(dir, "shared-cache")
	callsPath := filepath.Join(dir, "calls")
	cmd.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"GOCACHE="+cache, "GOFLAGS=-p=1 -trimpath", "VET_CALLS="+callsPath, "LOCAL_GO_TEST_JOBS=1")
	out, err := cmd.CombinedOutput()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 23 || strings.Contains(string(out), "stale-checkout") ||
		!strings.Contains(string(out), "current-checkout-stdout") || !strings.Contains(string(out), "current-checkout-stderr") {
		t.Fatalf("current checkout diagnostics/exit were lost: %v\n%s", err, out)
	}
	calls, err := os.ReadFile(callsPath)
	want := "vet -trimpath ./... GOCACHE=" + cache + " GOFLAGS=-p=1 -trimpath\n" +
		"vet -trimpath=false ./... GOCACHE=" + cache + " GOFLAGS=-p=1 -trimpath\n"
	if err != nil || string(calls) != want {
		t.Fatalf("cache/flags changed or fallback missing: %v\n%s\nwant:\n%s", err, calls, want)
	}
}

func TestCIVetCachePreservesDiagnostics(t *testing.T) {
	f := newVetCacheFixture(t)
	for _, tc := range []struct {
		name, source, finding string
	}{
		{"clean", "func Sum(a, b int) int { return a + b }\n", ""},
		{"shift", "func Shift(v uint32) uint32 { return v << 32 }\n", "too small for shift"},
		{"assignment", "func Self(v int) int { v = v; return v }\n", "self-assignment"},
		{"struct-tag", "type Row struct { Value string `json:\"broken` }\n", "struct field tag"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Warm the first path before checking the second, including success ->
			// finding transitions: a cache hit must never hide a new diagnostic.
			for _, dir := range []string{f.first, f.second} {
				f.source(t, dir, tc.source)
				before, beforeErr := f.run(t, dir, false, false)
				after, afterErr := f.run(t, dir, true, false)
				wantCode := 0
				if tc.finding != "" {
					wantCode = 1
				}
				for _, err := range []error{beforeErr, afterErr} {
					code := 0
					if err != nil {
						exit, ok := err.(*exec.ExitError)
						if !ok {
							t.Fatal(err)
						}
						code = exit.ExitCode()
					}
					if code != wantCode {
						t.Fatalf("exit = %d, want %d\nuntrimmed:\n%s\nlane:\n%s", code, wantCode, before, after)
					}
				}
				if before != after || (tc.finding == "" && after != "") || (tc.finding != "" && !strings.Contains(after, tc.finding)) {
					t.Fatalf("diagnostics changed\nuntrimmed:\n%s\nlane:\n%s", before, after)
				}
			}
		})
	}
}
