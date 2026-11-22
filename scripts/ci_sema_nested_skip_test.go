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
