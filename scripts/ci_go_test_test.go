package scripts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCIGoTestWrapperIsWired(t *testing.T) {
	script, err := os.ReadFile("ci-go-test.sh")
	if err != nil {
		t.Fatalf("read ci-go-test.sh: %v", err)
	}
	scriptText := string(script)
	for _, want := range []string{
		`export GOMAXPROCS="${GOMAXPROCS:-2}"`,
		"run_with_heartbeat",
		`set -o pipefail`,
		"tee",
		"./internal/apextest",
		"./internal/gladecli",
		"./internal/sema",
		"go test -v",
	} {
		if !strings.Contains(scriptText, want) {
			t.Fatalf("ci-go-test.sh missing %q", want)
		}
	}

	workflowPath := filepath.Join("..", ".github", "workflows", "ci.yml")
	workflow, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read %s: %v", workflowPath, err)
	}
	workflowText := string(workflow)
	for _, want := range []string{
		"timeout-minutes: 30",
		"timeout-minutes: 75",
		"GOMAXPROCS: \"2\"",
		"actions/checkout@v6",
		"actions/setup-go@v6",
		"actions/setup-node@v6",
		"scripts/ci-go-test.sh test",
		"scripts/ci-go-test.sh race",
	} {
		if !strings.Contains(workflowText, want) {
			t.Fatalf("ci.yml missing %q", want)
		}
	}
}
