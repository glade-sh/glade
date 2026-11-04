package apextest

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestC14GapGuardRequiresExactLeaf(t *testing.T) {
	allowed := map[string]bool{"TestExample/62.0/gap": true, "TestExample/67.0/gap": true}
	for _, tc := range []struct {
		name string
		want bool
	}{
		{"TestExample/62.0/gap", true}, {"TestExample/67.0/gap", true},
		{"TestExample/62.0/control", false}, {"TestExample/62.0/gap/nested", false},
		{"TestExample", false}, {"TestExample/66.0/gap", false},
	} {
		if got := case345GapAllowed(allowed, tc.name); got != tc.want {
			t.Errorf("%q: got %t, want %t", tc.name, got, tc.want)
		}
	}
}

func TestC14GapGuardRejectsUnregistered(t *testing.T) {
	if os.Getenv("GLADE_CASE345_GUARD_CHILD") == "1" {
		case345Skip(t)
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestC14GapGuardRejectsUnregistered$")
	cmd.Env = append(os.Environ(), "GLADE_CASE345_GUARD_CHILD=1")
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "unregistered native gap route: TestC14GapGuardRejectsUnregistered") || strings.Contains(string(out), "--- SKIP") {
		t.Fatalf("unexpected unregistered route result: err=%v output=%s", err, out)
	}
}
