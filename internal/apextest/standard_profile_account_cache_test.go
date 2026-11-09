package apextest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestStandardProfileAccountCachedRoutes verifies that cached project runners
// retain the captured runAs denial, including after a disk cache restore.
func TestStandardProfileAccountCachedRoutes(t *testing.T) {
	t.Cleanup(EnableDiskCacheForTesting())
	t.Cleanup(InvalidateRuntimeCaches)
	source, err := os.ReadFile(filepath.Join("testdata", "conformance", "standard_profile_account", "ProfileProbe.cls"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			InvalidateRuntimeCaches()
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			classes := filepath.Join(root, "force-app", "main", "default", "classes")
			writeFile(t, filepath.Join(classes, "ProfileProbe.cls"), string(source))
			writeFile(t, filepath.Join(classes, "ProfileProbe.cls-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?><ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			index := loadTestIndex(t, root)
			for pass := 0; pass < 3; pass++ {
				if pass == 2 {
					InvalidateRuntimeCaches()
				}
				ResetPerfCounters()
				run := Run(index, Options{SelectedClasses: []string{"ProfileProbe"}, Parallelism: 1, PerfCounters: true})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
					t.Fatalf("missing observation: %+v", run.Summary())
				}
				message := run.Suites[0].Cases[0].Problem.Message
				_, payload, ok := strings.Cut(message, "PROFILE_CAPTURE:")
				var got struct {
					Profiles []struct {
						Name      string `json:"name"`
						Account   bool   `json:"account"`
						NameField bool   `json:"nameField"`
					} `json:"profiles"`
				}
				if !ok || json.Unmarshal([]byte(payload), &got) != nil {
					t.Fatalf("invalid observation: %s", message)
				}
				denied := map[string]bool{"Work.com Only User": false, "Identity User": false, "Force.com - Free User": false, "Minimum Access - Salesforce": false, "Minimum Access - API Only Integrations": false}
				for _, p := range got.Profiles {
					if _, ok := denied[p.Name]; ok {
						if p.Account || p.NameField {
							t.Errorf("pass %d %s: Account=%t Name=%t; native false", pass, p.Name, p.Account, p.NameField)
						}
						denied[p.Name] = true
					}
				}
				for name, seen := range denied {
					if !seen {
						t.Errorf("pass %d missing profile %s", pass, name)
					}
				}
				phases := SnapshotPerfCounters().Phases
				if pass == 1 && phases.MemoryCacheHits != 1 {
					t.Fatalf("no memory cache hit: %+v", phases)
				}
				if pass == 2 && phases.DiskCacheHits != 1 {
					t.Fatalf("no disk cache hit: %+v", phases)
				}
			}
		})
	}
}
