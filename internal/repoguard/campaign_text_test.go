package repoguard

import (
	"strings"
	"testing"
)

// Maintenance text (private worktree paths, host and runner paths) and
// process documents do not belong in the product tree or its test data.
var campaignTextTokens = []string{
	"/private/" + "tmp/",
	"/Volumes/" + "Photos",
	"glade-" + "runner",
}

const conformanceTestdataDir = "internal/apextest/testdata/conformance/"

// campaignTextProblems reports why a tracked file should not be in the tree.
func campaignTextProblems(rel, text string) []string {
	var problems []string
	if strings.HasPrefix(rel, "internal/") || strings.HasPrefix(rel, "cmd/") {
		for _, token := range campaignTextTokens {
			if strings.Contains(text, token) {
				problems = append(problems, "contains "+token)
			}
		}
	}
	if strings.HasPrefix(rel, conformanceTestdataDir) && strings.HasSuffix(rel, ".md") && !strings.Contains(rel, "/customMetadata/") {
		problems = append(problems, "is a process document; conformance test data holds fixtures only")
	}
	return problems
}

func TestNoCampaignTextInSourceAndTestdata(t *testing.T) {
	root := repoRoot(t)
	for _, file := range repoRegularFiles(t, root) {
		for _, problem := range campaignTextProblems(file.rel, file.text) {
			t.Errorf("%s %s", file.rel, problem)
		}
	}
}

func TestCampaignTextProblems(t *testing.T) {
	for _, tc := range []struct {
		name, rel, text string
		want            int
	}{
		{"clean source", "internal/vm/vm.go", "package vm\n", 0},
		{"private worktree path", "internal/vm/vm.go", "// see " + "/private/" + "tmp/x\n", 1},
		{"host volume path in test data", "internal/apextest/testdata/conformance/a.json", `"/Volumes/` + `Photos/x"`, 1},
		{"runner home path", "internal/apextest/testdata/conformance/a.json", "/home/runner/glade-" + "runner/tmp/x", 1},
		{"process document", "internal/apextest/testdata/conformance/set_null_numeric/HANDOFF.md", "notes", 1},
		{"custom metadata fixture", "internal/apextest/testdata/conformance/ns/fixture/force-app/main/default/customMetadata/Flag.Seed.md", "", 0},
		{"docs may keep their own paths", "docs/ops.md", "/Volumes/" + "Photos", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := len(campaignTextProblems(tc.rel, tc.text)); got != tc.want {
				t.Fatalf("problems = %d, want %d", got, tc.want)
			}
		})
	}
}
