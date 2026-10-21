package apextest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/testreport"
)

func TestEventNumberDeliveryNativeConformance(t *testing.T) {
	dir := "testdata/conformance/event_number_delivery"
	var files map[string]string
	raw, err := os.ReadFile(filepath.Join(dir, "files.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &files); err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(dir, "api"+api[:2], "org.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
			if len(lines) != 2 {
				t.Fatal("missing native observation")
			}
			fields := strings.SplitN(lines[1], "\t", 3)
			if len(fields) != 3 || fields[0] != "observe" || fields[1] != "assertion" {
				t.Fatal("invalid native observation")
			}
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			for path, body := range files {
				writeFile(t, filepath.Join(root, "force-app/main/default", path), strings.ReplaceAll(body, "{{API}}", api))
			}
			run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
			seen := 0
			for _, suite := range run.Suites {
				for _, result := range suite.Cases {
					seen++
					if result.Status != testreport.StatusFail || result.Problem == nil {
						t.Fatalf("missing native observation assertion: %+v", result)
					}
					_, actual, ok := strings.Cut(result.Problem.Message, "NATIVE|")
					if !ok || actual != fields[2] {
						t.Fatalf("native <%s>; Glade <%s>", fields[2], result.Problem.Message)
					}
				}
			}
			if seen != 1 {
				t.Fatalf("want one observation, got %d: %+v", seen, run.Summary())
			}
		})
	}
}
