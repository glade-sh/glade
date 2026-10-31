package visualforce

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// The controller compilation gate consumes ordinary named Apex. Its sources
// correspond to the named deployment rows in sema/testdata/rtem_publish_route_twins.
func TestRTEMPublishControllerCompilation(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct {
			name, body string
			reject     bool
		}{
			{"scalar", "EventBus.publish(new FileEvent());", false},
			{"callback", "EventBus.publish(new FileEvent(), null);", false},
			{"ordinary_record", "EventBus.publish(new Account());", true},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
				writeFile(t, filepath.Join(root, "force-app/main/default/classes/ProbeController.cls"), "public class ProbeController { public void run() { "+tc.body+" } }")
				writeFile(t, filepath.Join(root, "force-app/main/default/classes/ProbeController.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
				writeFile(t, filepath.Join(root, "force-app/main/default/pages/Probe.page"), `<apex:page controller="ProbeController"/>`)
				p, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				diagnostics, err := ControllerCompileDiagnostics(p)
				if err != nil {
					t.Fatal(err)
				}
				rejected := false
				for _, d := range diagnostics {
					if d.Code == "GLADESEMA028" {
						rejected = true
					}
				}
				if rejected != tc.reject {
					t.Fatalf("reject=%v want %v diagnostics=%v", rejected, tc.reject, diagnostics)
				}
			})
		}
	}
}
