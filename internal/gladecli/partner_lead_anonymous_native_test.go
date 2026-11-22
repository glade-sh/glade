package gladecli

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// NATIVE partner_lead_anonymous_access/org.tsv:2-5 anonymous APIs 62.0 and 67.0.
// API 68 and the additional selected field preserve the uncaptured base shape.
func TestPartnerLeadAnonymousAccessNativeConformance(t *testing.T) {
	base := filepath.Join(criteriaSharingFixtures, "partner_lead_anonymous_access")
	rows := readCriteriaTSV(t, filepath.Join(base, "org.tsv"))
	for _, tc := range []struct {
		name, api         string
		extra, uncaptured bool
	}{
		{"62.0", "62.0", false, false},
		{"67.0", "67.0", false, false},
		{"68.0-uncaptured", "68.0", false, true},
		{"67.0-extra-field", "67.0", true, true},
	} {
		api := tc.api
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			for _, tree := range []string{"project", "local-setup"} {
				source := filepath.Join(base, tree)
				if err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
					if err != nil || entry.IsDir() {
						return err
					}
					b, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					rel, err := filepath.Rel(source, path)
					if err != nil {
						return err
					}
					writeTestFile(t, filepath.Join(root, "force-app/main/default", rel), string(b))
					return nil
				}); err != nil {
					t.Fatal(err)
				}
			}
			config, err := os.ReadFile(filepath.Join(base, "project-scratch-def.json"))
			if err != nil {
				t.Fatal(err)
			}
			writeTestFile(t, filepath.Join(root, "config/project-scratch-def.json"), string(config))
			project := func(version string) {
				writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+version+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			}
			db := filepath.Join(root, "state.sqlite")
			act := func(name string) string {
				t.Helper()
				script, err := os.ReadFile(filepath.Join(base, name+".apex"))
				if err != nil {
					t.Fatal(err)
				}
				if name == "observe" && tc.extra {
					script = []byte(strings.ReplaceAll(string(script), "HasEditAccess FROM", "HasEditAccess,HasDeleteAccess FROM"))
				}
				if name == "setup-records" {
					script = []byte(strings.Replace(string(script), "a.IsPartner=true;", "a.put('IsPartner',true);", 1))
				}
				if name != "observe" {
					script = append(script, []byte("\nSystem.debug('OBS|done');")...)
				}
				return runCriteriaExec(t, root, db, string(script))
			}
			project("67.0")
			for _, step := range []string{"setup-user", "setup-records", "setup-reader"} {
				act(step)
			}
			for _, phase := range []string{"before", "after"} {
				if phase == "after" {
					project("67.0")
					act("grant")
				}
				project(api)
				want := ""
				for _, row := range rows {
					if row["id"] == phase && row["api"] == api && strings.EqualFold(row["route"], "anonymous") {
						want = row["outcome"]
					}
				}
				if tc.uncaptured {
					want = "first=0:missing:missing|second=0:missing:missing"
				} else if want == "" {
					t.Fatalf("missing native %s %s observation", api, phase)
				}
				if got := act("observe"); got != want {
					t.Errorf("%s: Glade=%q native=%q", phase, got, want)
				}
			}
		})
	}
}
