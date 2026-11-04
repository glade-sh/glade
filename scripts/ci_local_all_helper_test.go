package scripts

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCILocalAllApexHelperSixShards(t *testing.T) {
	testCILocalAllApexHelper(t, 6)
}

func TestCILocalAllApexHelperTwelveShards(t *testing.T) {
	testCILocalAllApexHelper(t, 12)
}

func testCILocalAllApexHelper(t *testing.T, count int) {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "scripts")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("apextest-shards.sh")
	if err != nil {
		t.Fatal(err)
	}
	writeApexFixtureExecutable(t, filepath.Join(dir, "apextest-shards.sh"), string(data))
	writeApexFixtureExecutable(t, filepath.Join(dir, "ci-go-test.sh"), "#!/bin/bash\nprintf '%s HOME=%s\\n' \"$*\" \"${GLADE_HOME:-}\" >>\"$CALLS\"\n")
	calls := filepath.Join(root, "calls")
	cmd := exec.Command("bash", filepath.Join(dir, "apextest-shards.sh"), filepath.Join(root, "evidence"))
	cmd.Env = append(os.Environ(), fmt.Sprintf("CI_APEXTEST_SHARDS=%d", count), "GLADE_APEXTEST_SHARD_JOBS=1", "CALLS="+calls)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("helper: %v %s", err, out)
	}
	b, err := os.ReadFile(calls)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "apex-history-refresh") {
		t.Errorf("missing history refresh: %s", b)
	}
	for i := 0; i < count; i++ {
		want := fmt.Sprintf("apex-shard %d HOME=%s", i, filepath.Join(root, "evidence", fmt.Sprintf("shard-%d", i), "glade-home"))
		if !strings.Contains(string(b), want) {
			t.Errorf("missing isolated home %s: %s", want, b)
		}
	}
}
