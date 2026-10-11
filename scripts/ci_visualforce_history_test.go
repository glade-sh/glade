package scripts

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func visualforceHistoryFixture(t *testing.T) (string, []string) {
	t.Helper()
	root := t.TempDir()
	names := visualforceFixtureNames(8)
	shards := make([]map[string]any, 4)
	for index := range shards {
		tests := names[index*2 : index*2+2]
		shards[index] = map[string]any{"index": index, "tests": tests,
			"estimatedDurationMillis": 0, "regex": "^(?:" + strings.Join(tests, "|") + ")$"}
	}
	plan := map[string]any{"version": 1, "package": visualforceFixturePackage, "historyUsed": false, "shards": shards}
	dirs := make([]string, 4)
	for index := range dirs {
		dirs[index] = filepath.Join(root, fmt.Sprintf("shard-%d", index))
		if err := os.Mkdir(dirs[index], 0o700); err != nil {
			t.Fatal(err)
		}
		tests := names[index*2 : index*2+2]
		for name, data := range map[string]string{
			"discovery.txt": strings.Join(names, "\n") + "\n",
			"events.json":   visualforceFixtureEvents(tests),
		} {
			if err := os.WriteFile(filepath.Join(dirs[index], name), []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		writeJSONFixture(t, filepath.Join(dirs[index], "plan.json"), plan)
		writeJSONFixture(t, filepath.Join(dirs[index], "selected-shard.json"), shards[index])
		writeJSONFixture(t, filepath.Join(dirs[index], "validation-summary.json"), map[string]any{
			"valid": true, "expected": tests, "passed": tests, "errors": []string{}, "deferred": []string{}})
	}
	return filepath.Join(root, "history.json"), dirs
}

func runVisualforceHistory(t *testing.T, output string, dirs []string) (string, error) {
	t.Helper()
	args := append([]string{"ci-go-test.sh", "visualforce-history-refresh", output}, dirs...)
	out, err := exec.Command("bash", args...).CombinedOutput()
	return string(out), err
}

func assertVisualforceHistory(t *testing.T, output string, names []string) {
	t.Helper()
	data, err := os.ReadFile(output)
	var history struct {
		Version  int    `json:"version"`
		Package  string `json:"package"`
		Complete bool   `json:"complete"`
		Tests    []struct {
			Name     string `json:"name"`
			Duration int64  `json:"durationMillis"`
		} `json:"tests"`
	}
	if err != nil || json.Unmarshal(data, &history) != nil || history.Version != 1 || history.Package != visualforceFixturePackage || !history.Complete {
		t.Fatalf("invalid complete Visualforce history: %s %v", data, err)
	}
	var observed []string
	for _, test := range history.Tests {
		observed = append(observed, test.Name)
		if test.Duration < 0 {
			t.Fatalf("negative duration for %s", test.Name)
		}
	}
	if !reflect.DeepEqual(observed, names) {
		t.Fatalf("history test set=%v want=%v", observed, names)
	}
}

func TestCIVisualforceHistoryRejectsInvalidUnion(t *testing.T) {
	packagePass := fmt.Sprintf("{\"Action\":\"pass\",\"Package\":%q,\"Elapsed\":1}\n", visualforceFixturePackage)
	testPass := fmt.Sprintf("{\"Action\":\"pass\",\"Package\":%q,\"Test\":\"TestVisualforce007\",\"Elapsed\":100}\n", visualforceFixturePackage)
	for _, fixture := range []struct{ name, file, old, replacement string }{
		{"inconsistent plans", "plan.json", `"historyUsed": false`, `"historyUsed": true`},
		{"inconsistent discoveries", "discovery.txt", "TestVisualforce007", "TestOther"},
		{"inconsistent selection", "selected-shard.json", "TestVisualforce007", "TestOther"},
		{"wrong shard index", "selected-shard.json", `"index": 3`, `"index": 0`},
		{"extra terminal", "events.json", packagePass, strings.Replace(testPass, "TestVisualforce007", "TestOther", 1) + packagePass},
		{"missing terminal", "events.json", testPass, ""},
		{"skipped terminal", "events.json", `"Action":"pass"`, `"Action":"skip"`},
		{"failed terminal", "events.json", `"Action":"pass"`, `"Action":"fail"`},
		{"missing package terminal", "events.json", packagePass, ""},
		{"duplicate package terminal", "events.json", packagePass, packagePass + packagePass},
		{"skipped package terminal", "events.json", packagePass, strings.Replace(packagePass, `"pass"`, `"skip"`, 1)},
		{"failed package terminal", "events.json", packagePass, strings.Replace(packagePass, `"pass"`, `"fail"`, 1)},
		{"wrong package", "events.json", visualforceFixturePackage, semaFixturePackage},
		{"invalid summary", "validation-summary.json", `"valid": true`, `"valid": false`},
		{"forged deferral", "validation-summary.json", `"deferred": []`, `"deferred": ["TestVisualforce007"]`},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			output, dirs := visualforceHistoryFixture(t)
			path := filepath.Join(dirs[3], fixture.file)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), fixture.old) {
				t.Fatalf("fixture does not contain %q", fixture.old)
			}
			if err := os.WriteFile(path, []byte(strings.Replace(string(data), fixture.old, fixture.replacement, 1)), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(output, []byte("stale"), 0o600); err != nil {
				t.Fatal(err)
			}
			if out, err := runVisualforceHistory(t, output, dirs); err == nil {
				t.Fatalf("accepted invalid union: %s", out)
			}
			if _, err := os.Stat(output); !os.IsNotExist(err) {
				t.Fatalf("stale output survived rejection: %v", err)
			}
		})
	}
	for _, action := range []string{"skip", "fail"} {
		t.Run("nested "+action, func(t *testing.T) {
			output, dirs := visualforceHistoryFixture(t)
			path := filepath.Join(dirs[3], "events.json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			data = append(data, []byte(fmt.Sprintf("{\"Action\":%q,\"Package\":%q,\"Test\":\"TestVisualforce007/browser\"}\n", action, visualforceFixturePackage))...)
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}
			if out, err := runVisualforceHistory(t, output, dirs); err == nil {
				t.Fatalf("accepted nested %s under passing parent: %s", action, out)
			}
		})
	}
	for _, count := range []int{0, 3, 5} {
		output, dirs := visualforceHistoryFixture(t)
		dirs = append(dirs, dirs[0])
		if out, err := runVisualforceHistory(t, output, dirs[:count]); err == nil {
			t.Fatalf("accepted %d workers: %s", count, out)
		}
	}
	output, dirs := visualforceHistoryFixture(t)
	dirs[3] = dirs[2]
	if out, err := runVisualforceHistory(t, output, dirs); err == nil {
		t.Fatalf("accepted duplicate worker artifact: %s", out)
	}
}

func TestCIVisualforceHistoryBootstrapValidatesRawSerialEvidence(t *testing.T) {
	names := visualforceFixtureNames(8)
	valid := visualforceFixtureEvents(names)
	packagePass := fmt.Sprintf("{\"Action\":\"pass\",\"Package\":%q,\"Elapsed\":1}\n", visualforceFixturePackage)
	testPass := fmt.Sprintf("{\"Action\":\"pass\",\"Package\":%q,\"Test\":\"TestVisualforce000\",\"Elapsed\":100}\n", visualforceFixturePackage)
	for _, fixture := range []struct {
		name, events string
		valid        bool
	}{
		{"passing unfiltered predecessor", valid, true},
		{"missing events", "", false},
		{"no tests", packagePass, false},
		{"missing test terminal", strings.Replace(valid, testPass, "", 1), false},
		{"duplicate test terminal", valid + testPass, false},
		{"top-level skip", strings.Replace(valid, testPass, strings.Replace(testPass, `"pass"`, `"skip"`, 1), 1), false},
		{"top-level failure", strings.Replace(valid, testPass, strings.Replace(testPass, `"pass"`, `"fail"`, 1), 1), false},
		{"nested skip", valid + fmt.Sprintf("{\"Action\":\"skip\",\"Package\":%q,\"Test\":\"TestVisualforce000/browser\"}\n", visualforceFixturePackage), false},
		{"nested failure", valid + fmt.Sprintf("{\"Action\":\"fail\",\"Package\":%q,\"Test\":\"TestVisualforce000/browser\"}\n", visualforceFixturePackage), false},
		{"missing package", strings.Replace(valid, packagePass, "", 1), false},
		{"duplicate package", valid + packagePass, false},
		{"skipped package", strings.Replace(valid, packagePass, strings.Replace(packagePass, `"pass"`, `"skip"`, 1), 1), false},
		{"failed package", strings.Replace(valid, packagePass, strings.Replace(packagePass, `"pass"`, `"fail"`, 1), 1), false},
		{"wrong package", strings.Replace(valid, visualforceFixturePackage, semaFixturePackage, 1), false},
		{"malformed JSON", valid + "{broken\n", false},
		{"duplicate JSON key", strings.Replace(valid, `"Elapsed":100`, `"Elapsed":100,"Elapsed":1`, 1), false},
		{"negative duration", strings.Replace(valid, `"Elapsed":100`, `"Elapsed":-1`, 1), false},
		{"nonfinite duration", strings.Replace(valid, `"Elapsed":100`, `"Elapsed":NaN`, 1), false},
		{"boolean duration", strings.Replace(valid, `"Elapsed":100`, `"Elapsed":true`, 1), false},
		{"missing duration", strings.Replace(valid, `,"Elapsed":100`, "", 1), false},
		{"overflow duration", strings.Replace(valid, `"Elapsed":100`, `"Elapsed":1e30`, 1), false},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			root := t.TempDir()
			output, events := filepath.Join(root, "history.json"), filepath.Join(root, "events.json")
			for name, data := range map[string]string{output: "stale", events: fixture.events} {
				if err := os.WriteFile(name, []byte(data), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			out, err := exec.Command("bash", "ci-go-test.sh", "visualforce-history-bootstrap", output, events).CombinedOutput()
			if (err == nil) != fixture.valid {
				t.Fatalf("valid=%v exit=%v\n%s", fixture.valid, err, out)
			}
			if fixture.valid {
				assertVisualforceHistory(t, output, names)
			} else if _, err := os.Stat(output); !os.IsNotExist(err) {
				t.Fatalf("stale output survived rejection: %v", err)
			}
		})
	}
}
