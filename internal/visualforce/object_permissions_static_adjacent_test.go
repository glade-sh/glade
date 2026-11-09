package visualforce

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// Native controls: case289_followups/static_vf_controller/org.tsv.
func TestObjectPermissionsStaticAdjacentControllerCompilation(t *testing.T) {
	cases := map[string]string{
		"write":     `ObjectPermissions x=new ObjectPermissions();x.PermissionsViewAllFields=true;`,
		"qualified": `Object f=Schema.SObjectType.ObjectPermissions.fields.PermissionsViewAllFields;`,
	}
	for name, body := range cases {
		for _, api := range []string{"62.0", "67.0", "68.0"} {
			t.Run(name+"/"+api, func(t *testing.T) {
				p := writeControllerPageProject(t, `public class ProbeController { public String value {get;set;} public void observe(){`+body+`} }`)
				writeFile(t, filepath.Join(p.Root, "force-app/main/default/classes/ProbeController.cls-meta.xml"), fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api))
				writeStaticControllerPageVersions(t, p.Root, api)
				p, err := project.Load(p.Root)
				if err != nil {
					t.Fatal(err)
				}
				compilation := gateLikeCompilation(t, p)
				messages := strings.Join(diagnosticMessages(compilation), "\n")
				if api == "62.0" && !strings.Contains(messages, "Variable does not exist: PermissionsViewAllFields") {
					t.Fatalf("native rejection missing: %s", messages)
				}
				if api != "62.0" && messages != "" {
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
}
