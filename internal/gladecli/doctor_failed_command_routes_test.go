package gladecli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFailedTestCommandOrgSetupEvidenceRoutes(t *testing.T) {
	for _, name := range []string{"IdentifierOnlyUse", "RecordTypeAliasUse"} {
		t.Run("testCommand/"+name, func(t *testing.T) {
			root := t.TempDir()
			source, err := os.ReadFile("../project/testdata/conformance/org_setup_evidence/project/force-app/main/default/classes/" + name + ".cls")
			if err != nil {
				t.Fatal(err)
			}
			writeDoctorFixture(t, root, map[string]string{
				"sfdx-project.json": `{"packageDirectories":[{"path":"force-app"}]}`,
				"force-app/main/default/classes/" + name + ".cls": string(source),
				"force-app/main/default/classes/FailureTest.cls":  "@IsTest private class FailureTest { @IsTest static void fails() { System.assert(false, 'synthetic failure'); } }",
			})
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), []string{"test", "--project", root, "--class", "FailureTest", "--no-cache", "--no-serve", "--no-progress", "--json"}, &stdout, &stderr)
			var result struct {
				Summary struct{ Failed, CompileErrors int } `json:"summary"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &result); err != nil || code == 0 || result.Summary.Failed != 1 || result.Summary.CompileErrors != 0 {
				t.Fatalf("expected executed assertion failure: exit=%d result=%+v err=%v\n%s\n%s", code, result, err, stdout.String(), stderr.String())
			}
			hinted := strings.Contains(stderr.String(), "Hint: some failures")
			if hinted != (name == "RecordTypeAliasUse") {
				t.Fatalf("test-command setup hint=%v:\n%s", hinted, stderr.String())
			}
		})
	}
}

func writeDoctorFixture(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, contents := range files {
		writeTestFile(t, filepath.Join(root, name), contents)
	}
}
