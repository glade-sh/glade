package apextest

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
)

// Run the owned native static-selection and member-access classes locally.
func TestRoad174ReviewStaticProject(t *testing.T) {
	for _, api := range []string{"62", "67"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			version := api + ".0"
			class := "Road174Review" + api
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, version))
			src, err := os.ReadFile(filepath.Join("testdata/conformance/road174_review_capture/project/force-app/main/default/classes", class+".cls"))
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "force-app/main/default/classes", class+".cls"), string(src))
			writeFile(t, filepath.Join(root, "force-app/main/default/classes", class+".cls-meta.xml"), "<ApexClass><apiVersion>"+version+"</apiVersion></ApexClass>")
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native class accepted: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = version
			for _, name := range []string{"Account", "Lead", "Campaign", "Contract", "Asset", "Product2", "Pricebook2", "Order", "OpportunityLineItem", "Opportunity"} {
				storage.EnsureStandardObject(&org, name)
			}
			base := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org, LinkProject: true})
			cases := Discover(index, Options{SelectedClasses: []string{class}, SelectedMethod: "check"})
			if len(cases) != 1 {
				t.Fatalf("discovered %d methods", len(cases))
			}
			methods, methodErrors := compileTestMethodsForIndex(&index, cases)
			programs, programErrors := compileTestInvokePrograms(cases)
			key := testCaseKey(cases[0])
			if methodErrors[key] != nil || programErrors[key] != nil {
				t.Fatalf("compile %v %v", methodErrors[key], programErrors[key])
			}
			machine := base.newMachine()
			machine.EnableTestContext()
			if err := machine.RegisterMethod(methods[key]); err != nil {
				t.Fatal(err)
			}
			if _, err := machine.ExecuteInClass(programs[key], cases[0].ClassName); err != nil {
				t.Fatal(err)
			}
		})
	}
}
