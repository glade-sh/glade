package visualforce_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/visualforce"
)

func TestVisualforceFormStandardControllerBuiltinActions(t *testing.T) {
	// Page messages's captured configuration establishes the standard-controller plus
	// extension boundary. The regression adds commands whose methods belong
	// to the standard controller, rather than inventing extension declarations.
	data, err := os.ReadFile("testdata/v03_salesforce.json")
	if err != nil {
		t.Fatal(err)
	}
	var table struct {
		Cases []v10Case `json:"cases"`
	}
	if err := json.Unmarshal(data, &table); err != nil {
		t.Fatal(err)
	}
	var fixture v10Case
	for _, row := range table.Cases {
		if row.ID == "c_diagnostic_standard_account_extension" {
			fixture = row
			break
		}
	}
	if fixture.ID == "" {
		t.Fatal("missing native standard-controller extension fixture")
	}
	for _, api := range []string{"59.0", "67.0"} {
		for _, tag := range []string{"commandButton", "commandLink"} {
			for _, action := range []string{"save", "quickSave", "cancel", "edit", "view", "delete"} {
				t.Run(api+"/"+tag+"/"+action, func(t *testing.T) {
					root := t.TempDir()
					for path, source := range fixture.Inputs[api].Files {
						if strings.HasSuffix(path, ".page") {
							command := `<apex:form><apex:` + tag + ` value="Run" action="{!` + action + `}"/></apex:form>`
							source = strings.Replace(source, "</apex:page>", command+"</apex:page>", 1)
						}
						fullPath := filepath.Join(root, path)
						if err := os.MkdirAll(filepath.Dir(fullPath), 0o700); err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(fullPath, []byte(source), 0o600); err != nil {
							t.Fatal(err)
						}
					}
					p, err := project.Load(root)
					if err != nil {
						t.Fatal(err)
					}
					if _, err := visualforce.LoadProject(p); err != nil {
						t.Fatalf("standard-controller action %s rejected with extension: %v", action, err)
					}
				})
			}
		}
	}
}
