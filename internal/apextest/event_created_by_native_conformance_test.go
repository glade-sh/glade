package apextest

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A platform event subscriber trigger sees the publishing user in CreatedById
// and a non-null CreatedDate. The native rows (api62/api67 org.tsv) record the
// Pass outcome of the same project in a scratch org. Inside System.runAs the
// published event carries the runAs user; an event published outside it keeps
// the publisher when delivery happens inside runAs.
func TestEventCreatedByNativeConformance(t *testing.T) {
	fixture := "testdata/conformance/event_created_by"
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(fixture, "api"+strings.TrimSuffix(api, ".0"), "org.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(raw), `"outcome":"Pass"`) {
				t.Fatalf("native row is not a pass: %s", raw)
			}
			root := t.TempDir()
			src := filepath.Join(fixture, "project")
			err = filepath.WalkDir(src, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				rel, err := filepath.Rel(src, path)
				if err != nil {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				text := string(body)
				if strings.HasSuffix(rel, "-meta.xml") || rel == "sfdx-project.json" {
					text = strings.ReplaceAll(text, "62.0", api)
				}
				writeFile(t, filepath.Join(root, rel), text)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if api != "62.0" {
				// admin_field_grant_sources (2026-11-05): from API 67 an administrator
				// reads or writes an ordinary custom field only through an assigned
				// permission set or a profile grant. The native EvtRunAs row passed at 67.0,
				// so its capture user held the fixture's QAll set; replay that grant.
				writeCapturedAssignments(t, root, "QAll")
			}
			index := loadTestIndex(t, root)
			report := RunContext(context.Background(), index, Options{NoDiskCache: true, Parallelism: 1})
			count := 0
			for _, suite := range report.Suites {
				for _, c := range suite.Cases {
					count++
					if string(c.Status) != "pass" {
						t.Fatalf("%s.%s: %s %+v", suite.Name, c.Name, c.Status, c.Problem)
					}
				}
			}
			if count != 4 {
				t.Fatalf("cases=%d", count)
			}
		})
	}
}
