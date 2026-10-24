package vm

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Run the real row dispatcher so the guard also catches rows that are read but
// skipped instead of exercised. Fixture mutations never touch the saved oracle.
func TestNativeUnitRegressionRowCoverage(t *testing.T) {
	const fixture = "testdata/unit_regressions/native_capture/org.tsv"
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	rows := readNativeUnitRegressionRows(t)
	if len(rows) < 2 {
		t.Fatal("coverage guard needs at least two captured rows")
	}

	run := func(t *testing.T, lines []string) (string, error) {
		t.Helper()
		root := t.TempDir()
		path := filepath.Join(root, fixture)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		binary, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, binary, "-test.run=^TestNativeUnitRegressionRows$", "-test.v", "-test.count=1")
		cmd.Dir = root
		output, err := cmd.CombinedOutput()
		if ctx.Err() != nil {
			t.Fatalf("row dispatcher timed out: %s", output)
		}
		return string(output), err
	}

	t.Run("every-captured-api-case-route", func(t *testing.T) {
		output, err := run(t, lines)
		if err != nil {
			t.Fatalf("row dispatcher failed: %v\n%s", err, output)
		}
		for _, row := range rows {
			name := "TestNativeUnitRegressionRows/" + row.api + "/" + row.name
			if !strings.Contains(output, "--- PASS: "+name+" (") {
				t.Errorf("native row %s/%s was not exercised successfully\n%s", row.api, row.name, output)
			}
		}
	})

	t.Run("duplicate-replacing-another-row", func(t *testing.T) {
		duplicate := append([]string(nil), lines...)
		duplicate[2] = duplicate[1]
		output, err := run(t, duplicate)
		if err == nil || !strings.Contains(output, "duplicate native row "+rows[0].api+"/"+rows[0].name) {
			t.Fatalf("duplicate native row accepted or rejected for the wrong reason: %v\n%s", err, output)
		}
	})

	t.Run("missing-api-twin", func(t *testing.T) {
		missing := append([]string{lines[0]}, lines[2:]...)
		output, err := run(t, missing)
		if err == nil || !strings.Contains(output, "missing native row "+rows[0].api+"/"+rows[0].name) {
			t.Fatalf("missing native API twin accepted or rejected for the wrong reason: %v\n%s", err, output)
		}
	})
}
