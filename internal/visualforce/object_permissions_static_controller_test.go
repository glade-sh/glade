package visualforce

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// Native named Apex compile controls: case289_followups/static-named.tsv.
func TestObjectPermissionsStaticControllerCompilation(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			p := writeControllerPageProject(t, `public class ProbeController { public String value { get; set; } public void observe() { Schema.SObjectField f=ObjectPermissions.PermissionsViewAllFields; } }`)
			writeFile(t, filepath.Join(p.Root, "force-app/main/default/classes/ProbeController.cls-meta.xml"), fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api))
			writeStaticControllerPageVersions(t, p.Root, api)
			p, err := project.Load(p.Root)
			if err != nil {
				t.Fatal(err)
			}
			compilation := gateLikeCompilation(t, p)
			messages := strings.Join(diagnosticMessages(compilation), "\n")
			if api == "62.0" {
				if !strings.Contains(messages, "Variable does not exist: PermissionsViewAllFields") {
					t.Fatalf("native rejection missing: %s", messages)
				}
			} else if messages != "" {
				t.Fatalf("native compilation accepted: %s", messages)
			}
			for pass := 0; pass < 2; pass++ {
				for _, pages := range [][]string{PageNames(p), PageNamesWithControllerCompilation(p, compilation)} {
					if api == "62.0" && !onlyPlain(pages) {
						t.Fatalf("rejected controller registered pages: %v", pages)
					}
					if api != "62.0" && len(pages) != 2 {
						t.Fatalf("accepted controller pages: %v", pages)
					}
				}
			}
		})
	}
}

func writeStaticControllerPageVersions(t *testing.T, root, apexAPI string) {
	t.Helper()
	pageAPI := apexAPI
	if apexAPI == "62.0" {
		pageAPI = "59.0"
	}
	for _, name := range []string{"Probe", "Plain"} {
		writeFile(t, filepath.Join(root, "force-app/main/default/pages", name+".page-meta.xml"), fmt.Sprintf(`<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><label>%s</label></ApexPage>`, pageAPI, name))
	}
}
