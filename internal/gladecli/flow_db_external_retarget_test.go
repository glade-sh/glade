package gladecli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowDBExternalRootRetargetAndRemoval(t *testing.T) {
	source, err := os.ReadFile("../automation/testdata/flow_merge_format/Format_constant.flow-meta.xml")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ mode, suffix string }{{"retarget", ".flow"}, {"retarget", ".flow-meta.xml"}, {"remove", ".flow-meta.xml"}} {
		t.Run(tc.mode+tc.suffix, func(t *testing.T) {
			parent := t.TempDir()
			root := filepath.Join(parent, "project")
			writeTestProject(t, root)
			old := filepath.Join(parent, "old", "main/default/flows", "Old"+tc.suffix)
			next := filepath.Join(parent, "next", "main/default/flows", "New"+tc.suffix)
			writeTestFile(t, old, string(source))
			writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"../old","default":true}]}`)
			db := filepath.Join(root, "org.sqlite")
			store, org, err := openDBStore(db, root)
			if err != nil {
				t.Fatal(err)
			}
			account := org.Objects["Account"]
			account.Definition.FlowRules = append(account.Definition.FlowRules, storage.FlowRule{Name: "Runtime", File: filepath.Join(parent, "runtime", "Other.flow"), Active: true})
			org.Objects["Account"] = account
			if err := store.Save(org); err != nil {
				t.Fatal(err)
			}
			if err := store.Close(); err != nil {
				t.Fatal(err)
			}
			if tc.mode == "retarget" {
				writeTestFile(t, next, string(source))
				writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"../next","default":true}]}`)
			} else {
				writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
			}
			for _, phase := range []string{"cold", "warm"} {
				t.Run(phase, func(t *testing.T) {
					store, current, err := openDBStore(db, root)
					if err != nil {
						t.Fatal(err)
					}
					defer store.Close()
					counts := map[string]int{}
					for _, rule := range current.Objects["Account"].Definition.FlowRules {
						counts[strings.ToLower(rule.Name)]++
					}
					wantNew := 0
					if tc.mode == "retarget" {
						wantNew = 1
					}
					if counts["old"] != 0 || counts["new"] != wantNew || counts["runtime"] != 1 {
						t.Fatalf("flows=%v", counts)
					}
				})
			}
		})
	}
}
