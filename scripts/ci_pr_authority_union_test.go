package scripts

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCIPRAuthorityUnion(t *testing.T) {
	for _, tc := range []struct {
		name     string
		mutation string
		ok       bool
	}{
		{"complete", "", true}, {"missing worker", "missing", false},
		{"divergent discovery", "discovery", false}, {"divergent plan", "plan", false},
		{"duplicate terminal", "duplicate", false}, {"skipped terminal", "skip", false}, {"nested skip", "nested", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := semaPairFixture(t)
			root := filepath.Join(f.root, "input")
			plan := map[string]any{"include": []any{
				map[string]any{"shard": 20, "packages": []string{semaFixturePackage}, "pool": []string{}, "browser": false},
				map[string]any{"shard": 21, "packages": []string{semaFixturePackage}, "pool": []string{}, "browser": false},
			}}
			for i, src := range f.shards {
				dst := filepath.Join(root, fmt.Sprintf("pr-authority-1-1-%d", 20+i), "shard")
				if tc.mutation == "missing" && i == 1 {
					continue
				}
				if err := os.MkdirAll(dst, 0700); err != nil {
					t.Fatal(err)
				}
				entries, err := os.ReadDir(src)
				if err != nil {
					t.Fatal(err)
				}
				for _, entry := range entries {
					b, err := os.ReadFile(filepath.Join(src, entry.Name()))
					if err != nil {
						t.Fatal(err)
					}
					if i == 1 {
						if tc.mutation == "discovery" && entry.Name() == "discovery.txt" {
							b = append(b, []byte("TestZExtra\n")...)
						}
						if tc.mutation == "plan" && entry.Name() == "plan.json" {
							b = []byte(`{}`)
						}
						if tc.mutation == "duplicate" && entry.Name() == "events.json" {
							b = append(b, b...)
						}
						if tc.mutation == "nested" && entry.Name() == "events.json" {
							b = append(b, []byte(`{"Action":"skip","Package":"`+semaFixturePackage+`","Test":"TestHistory000/unknown"}`+"\n")...)
						}
						if tc.mutation == "skip" && entry.Name() == "events.json" {
							b = []byte(strings.Replace(string(b), `"Action":"pass"`, `"Action":"skip"`, 1))
						}
					}
					if err := os.WriteFile(filepath.Join(dst, entry.Name()), b, 0600); err != nil {
						t.Fatal(err)
					}
				}
				writeJSONFixture(t, filepath.Join(filepath.Dir(dst), "identity.json"), map[string]any{"shard": 20 + i})
			}
			planPath := filepath.Join(f.root, "plan.json")
			writeJSONFixture(t, planPath, plan)
			cmd := exec.Command("python3", "ci-pr-authority-union.py", planPath, root, filepath.Join(f.root, "out"))
			cmd.Env = append(os.Environ(), "GITHUB_EVENT_NAME=pull_request", "GITHUB_HEAD_REF=feature/test")
			out, err := cmd.CombinedOutput()
			if (err == nil) != tc.ok {
				t.Fatalf("union error=%v output=%s", err, out)
			}
			if tc.ok {
				data, err := os.ReadFile(filepath.Join(f.root, "out", "summary.json"))
				if err != nil || !json.Valid(data) {
					t.Fatalf("summary %s %v", data, err)
				}
			}
		})
	}
}

func TestCIPRAuthorityWorkflow(t *testing.T) {
	_, jobs := readCIWorkflow(t)
	for _, marker := range []string{"pr-authority-${{ github.run_id }}-${{ github.run_attempt }}-${{ matrix.shard }}", "ci-pr-packages.sh", "CI_SEMA_ARTIFACT_DIR"} {
		if !strings.Contains(jobs["pr-tests"], marker) {
			t.Errorf("PR worker missing %q", marker)
		}
	}
	if !strings.Contains(jobs["pr-authority-union"], "ci-pr-authority-union.py") || !strings.Contains(jobs["required-ci"], "      - pr-authority-union\n") {
		t.Fatal("PR authority union must gate Required CI")
	}
}

func TestCIPRPackageAudit(t *testing.T) {
	for _, tc := range []struct {
		name, action string
		ok           bool
	}{
		{"passed", "pass", true}, {"unexpected skip", "skip", false}, {"failed", "fail", false}, {"missing terminal", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			bin := filepath.Join(dir, "go")
			program := "#!/bin/bash\nif [[ $1 == list ]]; then printf '%s\\thas-tests\\n' github.com/glade-sh/glade/internal/example; exit; fi\nif [[ $* == *-list* ]]; then printf 'TestExample\\nok  \\tgithub.com/glade-sh/glade/internal/example\\t0.001s\\n'; exit; fi\nprintf '%s\\n' '{\"Action\":\"" + tc.action + "\",\"Package\":\"github.com/glade-sh/glade/internal/example\",\"Test\":\"TestExample\"}' '{\"Action\":\"pass\",\"Package\":\"github.com/glade-sh/glade/internal/example\"}'\n"
			if err := os.WriteFile(bin, []byte(program), 0700); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("bash", "ci-pr-packages.sh", filepath.Join(dir, "evidence"), "github.com/glade-sh/glade/internal/example")
			cmd.Dir = ".."
			// The wrapper resolves scripts from the repository root.
			cmd.Args[1] = "scripts/ci-pr-packages.sh"
			cmd.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"))
			out, err := cmd.CombinedOutput()
			if (err == nil) != tc.ok {
				t.Fatalf("audit error=%v output=%s", err, out)
			}
		})
	}
}

func TestCIPRDoctorAudit(t *testing.T) {
	for _, tc := range []struct {
		action string
		ok     bool
	}{{"pass", true}, {"skip", false}, {"fail", false}} {
		t.Run(tc.action, func(t *testing.T) {
			dir := t.TempDir()
			program := "#!/bin/bash\nprintf '%s\\n' '{\"Action\":\"" + tc.action + "\",\"Package\":\"github.com/glade-sh/glade/internal/gladecli\",\"Test\":\"TestDoctor\"}' '{\"Action\":\"pass\",\"Package\":\"github.com/glade-sh/glade/internal/gladecli\"}'\n"
			if err := os.WriteFile(filepath.Join(dir, "go"), []byte(program), 0700); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("bash", "scripts/ci-pr-doctor-test.sh", filepath.Join(dir, "evidence"), "^(TestDoctor)$")
			cmd.Dir = ".."
			cmd.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"))
			out, err := cmd.CombinedOutput()
			if (err == nil) != tc.ok {
				t.Fatalf("doctor audit error=%v output=%s", err, out)
			}
		})
	}
}

func TestCIPRDoctorUnion(t *testing.T) {
	const pkg = "github.com/glade-sh/glade/internal/gladecli"
	for _, omit := range []bool{false, true} {
		t.Run(fmt.Sprintf("omit=%v", omit), func(t *testing.T) {
			root := t.TempDir()
			input := filepath.Join(root, "input")
			shards := make([]map[string]any, 4)
			workers := make([]any, 0, 5)
			names := []string{"TestAlpha", "TestBeta", "TestDelta", "TestGamma"}
			for i, name := range names {
				shards[i] = map[string]any{"index": i, "tests": []string{name}, "estimatedDurationMillis": 0, "regex": "^(?:" + name + ")$"}
				workers = append(workers, map[string]any{"shard": 16 + i, "packages": []string{pkg}, "pool": []string{}, "browser": false})
			}
			workers = append(workers, map[string]any{"shard": 22, "packages": []string{pkg}, "pool": []string{}, "browser": false})
			for i := range shards {
				dir := filepath.Join(input, fmt.Sprintf("pr-authority-1-1-%d", 16+i))
				shard := filepath.Join(dir, "shard")
				if err := os.MkdirAll(shard, 0700); err != nil {
					t.Fatal(err)
				}
				writeJSONFixture(t, filepath.Join(dir, "identity.json"), map[string]any{"shard": 16 + i})
				writeJSONFixture(t, filepath.Join(shard, "plan.json"), map[string]any{"version": 1, "package": pkg, "historyUsed": false, "shards": shards})
				writeJSONFixture(t, filepath.Join(shard, "selected-shard.json"), shards[i])
				writeJSONFixture(t, filepath.Join(shard, "validation-summary.json"), map[string]any{"valid": true, "expected": []string{names[i]}, "passed": []string{names[i]}, "deferred": []string{}, "errors": []string{}})
				os.WriteFile(filepath.Join(shard, "discovery.txt"), []byte(strings.Join(names, "\n")+"\n"), 0600)
				os.WriteFile(filepath.Join(shard, "events.json"), []byte(fmt.Sprintf("{\"Action\":\"pass\",\"Package\":%q,\"Test\":%q,\"Elapsed\":0.1}\n{\"Action\":\"pass\",\"Package\":%q}\n", pkg, names[i], pkg)), 0600)
			}
			dir := filepath.Join(input, "pr-authority-1-1-22")
			doctor := filepath.Join(dir, "doctor")
			if err := os.MkdirAll(doctor, 0700); err != nil {
				t.Fatal(err)
			}
			writeJSONFixture(t, filepath.Join(dir, "identity.json"), map[string]any{"shard": 22})
			os.WriteFile(filepath.Join(doctor, "metadata.tsv"), []byte(pkg+"\thas-tests\n"), 0600)
			var events strings.Builder
			for i, name := range nodeIntegrationTests[pkg] {
				if omit && i == 0 {
					continue
				}
				fmt.Fprintf(&events, "{\"Action\":\"pass\",\"Package\":%q,\"Test\":%q}\n", pkg, name)
			}
			fmt.Fprintf(&events, "{\"Action\":\"pass\",\"Package\":%q}\n", pkg)
			os.WriteFile(filepath.Join(doctor, "events.json"), []byte(events.String()), 0600)
			planPath := filepath.Join(root, "plan.json")
			writeJSONFixture(t, planPath, map[string]any{"include": workers})
			cmd := exec.Command("python3", "ci-pr-authority-union.py", planPath, input, filepath.Join(root, "out"))
			out, err := cmd.CombinedOutput()
			if (err == nil) == omit {
				t.Fatalf("doctor union err=%v output=%s", err, out)
			}
		})
	}
}
