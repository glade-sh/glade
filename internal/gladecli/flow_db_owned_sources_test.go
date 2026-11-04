package gladecli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFlowDBRefreshProjectOwnedSources(t *testing.T) {
	source, err := os.ReadFile("../automation/testdata/flow_merge_format/Format_constant.flow-meta.xml")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, suffix    string
		external, extra bool
	}{
		{"internal_metadata", ".flow-meta.xml", false, false},
		{"internal_flow", ".flow", false, false},
		{"external_metadata", ".flow-meta.xml", true, false},
		{"external_flow", ".flow", true, false},
		{"extra_metadata", ".flow", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parent := t.TempDir()
			root := filepath.Join(parent, "project")
			writeTestProject(t, root)
			pkg := filepath.Join(root, "force-app")
			if tc.external {
				pkg = filepath.Join(parent, "outside")
				if tc.extra {
					writeTestFile(t, filepath.Join(root, "glade.yml"), "project:\n  extraMetadataDirs: [../outside]\n")
				} else {
					writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"../outside","default":true}]}`)
				}
			}
			dir := filepath.Join(pkg, "main/default/flows")
			old := filepath.Join(dir, "Tracked"+tc.suffix)
			writeTestFile(t, old, string(source))
			db := filepath.Join(root, "org.sqlite")
			store, org, err := openDBStore(db, root)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, rule := range org.Objects["Account"].Definition.FlowRules {
				if strings.EqualFold(rule.Name, "Tracked") {
					found = true
				}
			}
			if !found {
				t.Fatal("original Flow not loaded")
			}
			if err := store.Close(); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(old, filepath.Join(dir, "Renamed"+tc.suffix)); err != nil {
				t.Fatal(err)
			}
			for _, pass := range []string{"cold", "warm"} {
				t.Run(pass, func(t *testing.T) {
					s, current, err := openDBStore(db, root)
					if err != nil {
						t.Fatal(err)
					}
					defer s.Close()
					oldCount, newCount := 0, 0
					for _, rule := range current.Objects["Account"].Definition.FlowRules {
						if strings.EqualFold(rule.Name, "Tracked") {
							oldCount++
						}
						if strings.EqualFold(rule.Name, "Renamed") {
							newCount++
						}
					}
					if oldCount != 0 || newCount != 1 {
						t.Fatalf("after rename: old=%d new=%d", oldCount, newCount)
					}
				})
			}
		})
	}
}
