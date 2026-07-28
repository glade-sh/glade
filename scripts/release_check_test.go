package scripts

import (
	"os"
	"strings"
	"testing"
)

func TestReleaseCheckBoundsFullGoSuiteResources(t *testing.T) {
	data, err := os.ReadFile("release-check.sh")
	if err != nil {
		t.Fatal(err)
	}
	script := string(data)
	for _, want := range []string{
		`export GOMAXPROCS="${GOMAXPROCS:-2}"`,
		"go test -count=1 -p=1 ./...",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("release check missing resource bound %q:\n%s", want, script)
		}
	}
	if strings.Contains(script, "go test ./...") {
		t.Fatalf("release check retains an unbounded full Go suite:\n%s", script)
	}
	for _, want := range []string{"git diff --check", "npm test --prefix site", "npm run build --prefix site", "scripts/smoke.sh"} {
		if !strings.Contains(script, want) {
			t.Fatalf("release check dropped existing gate %q:\n%s", want, script)
		}
	}
}
