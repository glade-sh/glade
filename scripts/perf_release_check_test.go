package scripts

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type perfReleaseCheckRecord struct {
	SchemaVersion int      `json:"schema_version"`
	Label         string   `json:"label"`
	Command       []string `json:"command"`
	Cache         struct {
		Mode                          string `json:"mode"`
		CallerVerified                bool   `json:"caller_verified"`
		GoCacheEnvironmentOverride    bool   `json:"go_cache_environment_override"`
		NPMCacheEnvironmentOverride   bool   `json:"npm_cache_environment_override"`
		GladeCacheEnvironmentOverride bool   `json:"glade_cache_environment_override"`
	} `json:"cache"`
	Commit struct {
		SHA   string `json:"sha"`
		Dirty bool   `json:"dirty"`
	} `json:"commit"`
	Host struct {
		OS           string `json:"os"`
		Architecture string `json:"architecture"`
		CPUs         int    `json:"cpus"`
		MemoryBytes  int64  `json:"memory_bytes"`
	} `json:"host"`
	Toolchain struct {
		GoVersion        string `json:"go_version"`
		NodeVersion      string `json:"node_version"`
		NPMVersion       string `json:"npm_version"`
		GOMAXPROCS       string `json:"gomaxprocs"`
		SelectedJobCount *int   `json:"selected_job_count"`
	} `json:"toolchain"`
	Phases []struct {
		Name       string `json:"name"`
		ExitStatus int    `json:"exit_status"`
		Resources  struct {
			WallSeconds   float64 `json:"wall_seconds"`
			UserSeconds   float64 `json:"user_seconds"`
			SystemSeconds float64 `json:"system_seconds"`
			MaxRSSBytes   int64   `json:"max_rss_bytes"`
			FileInputs    *int64  `json:"file_inputs"`
			FileOutputs   *int64  `json:"file_outputs"`
		} `json:"resources"`
	} `json:"phases"`
}

func runPerfReleaseCheck(t *testing.T, label string, command ...string) (perfReleaseCheckRecord, string, error) {
	t.Helper()
	output := filepath.Join(t.TempDir(), "caller-selected", "measurements")
	args := []string{"perf-release-check.sh", "--label", label, "--output", output, "--"}
	args = append(args, command...)
	cmd := exec.Command("bash", args...)
	cmd.Env = append(os.Environ(), "GOMAXPROCS=3", "LOCAL_GO_TEST_JOBS=2")
	stdout, err := cmd.CombinedOutput()
	data, readErr := os.ReadFile(filepath.Join(output, "release-check.json"))
	if readErr != nil {
		t.Fatalf("measurement record: %v\n%s", readErr, stdout)
	}
	var record perfReleaseCheckRecord
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatalf("measurement JSON: %v\n%s", err, data)
	}
	return record, string(stdout), err
}

func TestPerfReleaseCheckRecordsStableHostCommandAndResources(t *testing.T) {
	record, stdout, err := runPerfReleaseCheck(t, "baseline-cold", "bash", "-c", "printf 'authority ran\\n'")
	if err != nil {
		t.Fatalf("measurement wrapper failed: %v\n%s", err, stdout)
	}
	if record.SchemaVersion != 1 || record.Label != "baseline-cold" {
		t.Fatalf("record identity = %#v", record)
	}
	if got, want := record.Command, []string{"bash", "-c", "printf 'authority ran\\n'"}; strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("command = %#v, want %#v", got, want)
	}
	if record.Commit.SHA == "" || record.Host.OS == "" || record.Host.Architecture == "" || record.Host.CPUs < 1 || record.Host.MemoryBytes < 1 {
		t.Fatalf("missing host or commit evidence: %#v", record)
	}
	if record.Toolchain.GoVersion == "" || record.Toolchain.NodeVersion == "" || record.Toolchain.NPMVersion == "" || record.Toolchain.GOMAXPROCS != "3" || record.Toolchain.SelectedJobCount == nil || *record.Toolchain.SelectedJobCount != 2 {
		t.Fatalf("missing toolchain evidence: %#v", record.Toolchain)
	}
	if len(record.Phases) != 1 {
		t.Fatalf("phase records = %#v", record.Phases)
	}
	phase := record.Phases[0]
	if phase.Name != "release-check" || phase.ExitStatus != 0 || phase.Resources.WallSeconds < 0 || phase.Resources.UserSeconds < 0 || phase.Resources.SystemSeconds < 0 || phase.Resources.MaxRSSBytes < 1 {
		t.Fatalf("invalid phase record: %#v", phase)
	}
	if !strings.Contains(stdout, "authority ran") || !strings.Contains(stdout, "[perf] release-check") {
		t.Fatalf("missing command or measurement output:\n%s", stdout)
	}
}

func TestPerfReleaseCheckPreservesFailureAndRecordsColdWarmAndNoGoCacheLabels(t *testing.T) {
	for _, label := range []string{"baseline-cold", "baseline-warm", "baseline-no-go-cache"} {
		record, stdout, err := runPerfReleaseCheck(t, label, "bash", "-c", "exit 23")
		if err == nil {
			t.Fatalf("%s failure was accepted:\n%s", label, stdout)
		}
		if exit := err.(*exec.ExitError).ExitCode(); exit != 23 {
			t.Fatalf("%s exit = %d, want 23\n%s", label, exit, stdout)
		}
		if record.Label != label || len(record.Phases) != 1 || record.Phases[0].ExitStatus != 23 {
			t.Fatalf("%s did not preserve failure record: %#v", label, record)
		}
	}
}

func TestPerfReleaseCheckUsesStableTimerLocaleWithoutChangingChildLocale(t *testing.T) {
	output := filepath.Join(t.TempDir(), "measurements")
	cmd := exec.Command("bash", "perf-release-check.sh", "--label", "locale-regression", "--cache-mode", "warm", "--output", output, "--", "bash", "-c", "test \"$LC_ALL\" = fr_FR.UTF-8")
	cmd.Env = append(os.Environ(), "LC_ALL=fr_FR.UTF-8")
	stdout, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("localized timer changed successful child status: %v\n%s", err, stdout)
	}
	data, err := os.ReadFile(filepath.Join(output, "release-check.json"))
	if err != nil {
		t.Fatal(err)
	}
	var record perfReleaseCheckRecord
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	if len(record.Phases) != 1 || record.Phases[0].ExitStatus != 0 {
		t.Fatalf("localized timer record = %#v", record.Phases)
	}
}

func TestPerfReleaseCheckRecordsCallerVerifiedCacheModeAndEnvironmentProvenance(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "measurements")
	cmd := exec.Command("bash", "perf-release-check.sh", "--label", "baseline-no-go-cache", "--cache-mode", "no-go-cache", "--output", output, "--", "true")
	cmd.Env = append(os.Environ(),
		"GOCACHE="+filepath.Join(dir, "isolated-go-cache"),
		"npm_config_cache="+filepath.Join(dir, "npm-cache"),
		"GLADE_CACHE_DIR="+filepath.Join(dir, "glade-cache"),
	)
	stdout, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cache provenance measurement failed: %v\n%s", err, stdout)
	}
	data, err := os.ReadFile(filepath.Join(output, "release-check.json"))
	if err != nil {
		t.Fatal(err)
	}
	var record perfReleaseCheckRecord
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	if record.Cache.Mode != "no-go-cache" || !record.Cache.CallerVerified || !record.Cache.GoCacheEnvironmentOverride || !record.Cache.NPMCacheEnvironmentOverride || !record.Cache.GladeCacheEnvironmentOverride {
		t.Fatalf("cache provenance = %#v", record.Cache)
	}
}

func TestPerfReleaseCheckRecordsAutomaticSelectedJobCount(t *testing.T) {
	output := filepath.Join(t.TempDir(), "measurements")
	cmd := exec.Command("bash", "perf-release-check.sh", "--label", "automatic-jobs", "--output", output, "--", "true")
	cmd.Env = append(os.Environ(), "LOCAL_GO_TEST_JOBS=auto")
	stdout, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("automatic job measurement failed: %v\n%s", err, stdout)
	}
	data, err := os.ReadFile(filepath.Join(output, "release-check.json"))
	if err != nil {
		t.Fatal(err)
	}
	var record perfReleaseCheckRecord
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	if record.Toolchain.SelectedJobCount == nil || *record.Toolchain.SelectedJobCount != 1 {
		t.Fatalf("automatic selected job count = %#v, want 1", record.Toolchain.SelectedJobCount)
	}
}

func TestPerfReleaseCheckDoesNotAlterCacheEnvironment(t *testing.T) {
	dir := t.TempDir()
	goCache := filepath.Join(dir, "go-cache")
	npmCache := filepath.Join(dir, "npm-cache")
	gladeCache := filepath.Join(dir, "glade-cache")
	for _, cache := range []string{goCache, npmCache, gladeCache} {
		if err := os.MkdirAll(cache, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(cache, "sentinel"), []byte(cache), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	output := filepath.Join(dir, "caller-selected-output")
	cmd := exec.Command("bash", "perf-release-check.sh", "--label", "cache-observation", "--output", output, "--", "bash", "-c", "test \"$GOCACHE\" = \"$EXPECTED_GOCACHE\" && test \"$npm_config_cache\" = \"$EXPECTED_NPM_CACHE\" && test \"$GLADE_CACHE_DIR\" = \"$EXPECTED_GLADE_CACHE\"")
	cmd.Env = append(os.Environ(),
		"GOCACHE="+goCache, "npm_config_cache="+npmCache, "GLADE_CACHE_DIR="+gladeCache,
		"EXPECTED_GOCACHE="+goCache, "EXPECTED_NPM_CACHE="+npmCache, "EXPECTED_GLADE_CACHE="+gladeCache,
	)
	stdout, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("wrapper changed cache environment: %v\n%s", err, stdout)
	}
	for _, cache := range []string{goCache, npmCache, gladeCache} {
		data, err := os.ReadFile(filepath.Join(cache, "sentinel"))
		if err != nil || string(data) != cache {
			t.Fatalf("cache sentinel %s = %q, %v", cache, data, err)
		}
	}
	if _, err := os.Stat(filepath.Join(output, "release-check.json")); err != nil {
		t.Fatalf("caller-selected output missing: %v", err)
	}

	script, err := os.ReadFile("perf-release-check.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"go clean", "npm cache clean", "rm -rf", "GOCACHE="} {
		if strings.Contains(string(script), forbidden) {
			t.Fatalf("measurement wrapper mutates caches with %q", forbidden)
		}
	}
}
