package runartifact

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOpenCreatesRunDirAndLatestPointer(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 5, 23, 14, 32, 10, 0, time.UTC)

	run, err := Open(root, "", now)
	if err != nil {
		t.Fatal(err)
	}
	if run.ID != "20260523-143210" {
		t.Fatalf("run ID = %q", run.ID)
	}
	if run.Dir != filepath.Join(root, "20260523-143210") {
		t.Fatalf("run dir = %q", run.Dir)
	}

	if err := run.WriteJSON("results.json", map[string]int{"passed": 3}); err != nil {
		t.Fatal(err)
	}
	if err := run.WriteText("summary.md", "Result: 3 passed\n"); err != nil {
		t.Fatal(err)
	}
	if err := run.WriteLatest(root, Latest{SummaryPath: run.Path("summary.md"), ResultsPath: run.Path("results.json")}); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(root, "latest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var latest Latest
	if err := json.Unmarshal(data, &latest); err != nil {
		t.Fatal(err)
	}
	if latest.RunID != run.ID || latest.RunDir != run.Dir {
		t.Fatalf("latest pointer = %+v", latest)
	}
	if _, err := os.Stat(filepath.Join(run.Dir, "results.json")); err != nil {
		t.Fatal(err)
	}
}

func TestOpenRejectsUnsafeRunID(t *testing.T) {
	_, err := Open(t.TempDir(), "../escape", time.Now())
	if err == nil {
		t.Fatal("expected unsafe run ID error")
	}
}

func TestCleanKeepsNewestRuns(t *testing.T) {
	root := t.TempDir()
	for _, id := range []string{"20260523-143210", "20260523-143211", "20260523-143212"} {
		run, err := Open(root, id, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if err := run.WriteText("summary.md", id); err != nil {
			t.Fatal(err)
		}
	}

	removed, err := Clean(root, 2)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	if _, err := os.Stat(filepath.Join(root, "20260523-143210")); !os.IsNotExist(err) {
		t.Fatalf("oldest run still exists or unexpected err: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "20260523-143212")); err != nil {
		t.Fatalf("newest run missing: %v", err)
	}
}
