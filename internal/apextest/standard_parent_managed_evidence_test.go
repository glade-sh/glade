package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type standardParentInstalledCapture struct {
	API                 string `json:"source_api"`
	SubscriberNamespace string `json:"subscriber_namespace"`
	Field               struct {
		DeveloperName, NamespacePrefix, ManageableState, TableEnumOrId string
	}
	Package struct {
		NamespacePrefix string
		IsManaged       bool
	}
	Classes []struct {
		Name, NamespacePrefix string
		APIVersion            float64 `json:"ApiVersion"`
	}
}

func (capture standardParentInstalledCapture) validate(api string) error {
	if capture.API != api || capture.Field.ManageableState != "installed" || !capture.Package.IsManaged {
		return fmt.Errorf("API %s requires an installed field from a managed package; got API %s, ManageableState=%q, IsManaged=%t", api, capture.API, capture.Field.ManageableState, capture.Package.IsManaged)
	}
	if capture.Field.TableEnumOrId != "Contact" || capture.Field.DeveloperName == "" || capture.Field.NamespacePrefix == "" || capture.Field.NamespacePrefix != capture.Package.NamespacePrefix {
		return fmt.Errorf("installed Contact field and package identities must agree")
	}
	if capture.SubscriberNamespace == "" || capture.SubscriberNamespace == capture.Package.NamespacePrefix {
		return fmt.Errorf("capture must execute in a subscriber namespace distinct from the provider")
	}
	seen := map[string]bool{}
	for _, class := range capture.Classes {
		if seen[class.Name] || (class.Name != "ManagedParentRouteProbe" && class.Name != "ManagedParentRouteProbeTest") || class.NamespacePrefix != capture.SubscriberNamespace || fmt.Sprintf("%.1f", class.APIVersion) != api {
			return fmt.Errorf("anonymous/named and isTest observer class identity or API differs")
		}
		seen[class.Name] = true
	}
	if len(seen) != 2 {
		return fmt.Errorf("both the named observer and isTest entry need native API evidence")
	}
	return nil
}

func requireStandardParentInstalledEvidence(t *testing.T) {
	t.Helper()
	root := filepath.Join("testdata", "conformance", "standard_parent_routes", "managed")
	body, err := os.ReadFile(filepath.Join(root, "installed-field.json"))
	if err != nil {
		t.Fatalf("managed routes require installed-field evidence: %v", err)
	}
	var evidence struct {
		Captures []standardParentInstalledCapture
		Sources  map[string]string `json:"source_sha256"`
	}
	if err := json.Unmarshal(body, &evidence); err != nil {
		t.Fatal(err)
	}
	if len(evidence.Captures) != 2 {
		t.Fatal("installed-field evidence must cover API 62 and 67")
	}
	classPath := "project/force-app/main/default/classes/ManagedParentRouteProbe.cls"
	for _, path := range []string{"anonymous.apex", classPath, "project/force-app/main/default/classes/ManagedParentRouteProbeTest.cls"} {
		body, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		if evidence.Sources[path] != fmt.Sprintf("%x", sha256.Sum256(body)) {
			t.Fatalf("installed-field capture source differs: %s", path)
		}
	}
	source, err := os.ReadFile(filepath.Join(root, classPath))
	if err != nil {
		t.Fatal(err)
	}
	for i, api := range []string{"62.0", "67.0"} {
		capture := evidence.Captures[i]
		if err := capture.validate(api); err != nil {
			t.Fatal(err)
		}
		leaf := capture.Field.NamespacePrefix + "__" + capture.Field.DeveloperName + "__c"
		// Bind the installed field to the parent projection and the local schema,
		// rather than accepting evidence for an unrelated installed component.
		if !strings.Contains(string(source), "Contact__r."+leaf) {
			t.Fatalf("installed field %s is absent from the captured parent route", leaf)
		}
		if _, err := os.Stat(filepath.Join(root, "project/force-app/main/default/objects/Contact/fields", leaf+".field-meta.xml")); err != nil {
			t.Fatal(err)
		}
	}
}

func TestStandardParentManagedAnonymousAndIsTestInstalledEvidence(t *testing.T) {
	requireStandardParentInstalledEvidence(t)
}

func TestStandardParentManagedEvidenceRejectsNamespaceOnlyAnonymousAndIsTestRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous_named", "isTest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				var valid standardParentInstalledCapture
				body := `{"source_api":"` + api + `","subscriber_namespace":"subscriber","field":{"DeveloperName":"Note","NamespacePrefix":"provider","ManageableState":"installed","TableEnumOrId":"Contact"},"package":{"NamespacePrefix":"provider","IsManaged":true},"classes":[{"Name":"ManagedParentRouteProbe","NamespacePrefix":"subscriber","ApiVersion":` + api + `},{"Name":"ManagedParentRouteProbeTest","NamespacePrefix":"subscriber","ApiVersion":` + api + `}]}`
				if err := json.Unmarshal([]byte(body), &valid); err != nil {
					t.Fatal(err)
				}
				if err := valid.validate(api); err != nil {
					t.Fatal(err)
				}
				mutations := map[string]func(*standardParentInstalledCapture){
					"namespace_only":    func(c *standardParentInstalledCapture) { c.Field.ManageableState = "unmanaged" },
					"provider_beta":     func(c *standardParentInstalledCapture) { c.Field.ManageableState = "beta" },
					"missing_state":     func(c *standardParentInstalledCapture) { c.Field.ManageableState = "" },
					"unmanaged_package": func(c *standardParentInstalledCapture) { c.Package.IsManaged = false },
					"wrong_package":     func(c *standardParentInstalledCapture) { c.Package.NamespacePrefix = "other" },
					"provider_org":      func(c *standardParentInstalledCapture) { c.SubscriberNamespace = "provider" },
					"wrong_parent":      func(c *standardParentInstalledCapture) { c.Field.TableEnumOrId = "Account" },
					"wrong_api":         func(c *standardParentInstalledCapture) { c.API = "61.0" },
					"missing_entry":     func(c *standardParentInstalledCapture) { c.Classes = c.Classes[:1] },
				}
				for name, mutate := range mutations {
					t.Run(name, func(t *testing.T) {
						capture := valid
						mutate(&capture)
						if err := capture.validate(api); err == nil {
							t.Fatal("invalid managed provenance was accepted")
						}
					})
				}
			})
		}
	}
}
