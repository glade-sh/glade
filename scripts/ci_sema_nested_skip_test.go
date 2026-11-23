package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCISemaHistoryRejectsNestedSkip(t *testing.T) {
	f := semaPairFixture(t)
	for i, dir := range f.shards {
		path := filepath.Join(dir, "events.json")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			data = append(data, []byte(`{"Action":"skip","Package":"`+semaFixturePackage+`","Test":"TestHistory000/unknown"}`+"\n")...)
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("bash", "ci-go-test.sh", "sema-history-refresh", f.shards[0], f.shards[1], f.output)
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("unexpected nested skip accepted: %s", out)
	}
}

func TestCISemaEquivalenceRejectsNestedSkip(t *testing.T) {
	f := semaPairFixture(t)
	full := filepath.Join(f.root, "full")
	if err := os.MkdirAll(full, 0700); err != nil {
		t.Fatal(err)
	}
	discovery, err := os.ReadFile(filepath.Join(f.shards[0], "discovery.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(full, "discovery.txt"), discovery, 0600); err != nil {
		t.Fatal(err)
	}
	var events []byte
	for _, dir := range f.shards {
		data, err := os.ReadFile(filepath.Join(dir, "events.json"))
		if err != nil {
			t.Fatal(err)
		}
		events = append(events, data...)
	}
	events = append(events, []byte(`{"Action":"skip","Package":"`+semaFixturePackage+`","Test":"TestHistory000/unknown"}`+"\n")...)
	if err := os.WriteFile(filepath.Join(full, "events.json"), events, 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", "ci-go-test.sh", "sema-equivalence", f.shards[0], f.shards[1], full, filepath.Join(f.root, "result.json"))
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("full oracle accepted nested skip: %s", out)
	}
}
