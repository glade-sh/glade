package gladecli

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// NATIVE partner_lead_anonymous_access/org.tsv:2-5 anonymous APIs 62.0 and 67.0.
func TestPartnerLeadAnonymousAccessNativeConformance(t *testing.T) {
	base := filepath.Join(criteriaSharingFixtures, "partner_lead_anonymous_access")
	rows := readCriteriaTSV(t, filepath.Join(base, "org.tsv"))
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
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
				if want == "" {
					t.Fatalf("missing native %s %s observation", api, phase)
				}
				if got := act("observe"); got != want {
					t.Errorf("%s: Glade=%q native=%q", phase, got, want)
				}
			}
		})
	}
}
