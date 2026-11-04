package gladecli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// A project-bound DB refresh must remove prior project-owned rules while
// preserving runtime-only rules and stored rows across cold and warm opens.
func TestFlowDBRefreshLifecycle(t *testing.T) {
	source, err := os.ReadFile("../automation/testdata/flow_merge_format/Format_constant.flow-meta.xml")
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"deleted", "inactive", "renamed", "moved", "invalidated"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			writeTestProject(t, root)
			dir := filepath.Join(root, "force-app/main/default/flows")
			path := filepath.Join(dir, "Tracked.flow-meta.xml")
			anchor := filepath.Join(dir, "Anchor.flow-meta.xml")
			writeTestFile(t, path, string(source))
			writeTestFile(t, anchor, strings.Replace(string(source), "<object>Account</object>", "<object>Contact</object>", 1))
			db := filepath.Join(root, "org.sqlite")
			store, org, err := openDBStore(db, root)
			if err != nil {
				t.Fatal(err)
			}
			account := org.Objects["Account"]
			account.Definition.FlowRules = append(account.Definition.FlowRules, storage.FlowRule{Name: "Unrelated", Active: true, TriggerType: "RecordBeforeSave"})
			account.Records[storage.ID("001000000000001AAA")] = storage.Record{Object: "Account", ID: storage.ID("001000000000001AAA"), Fields: map[string]storage.Value{"Name": storage.StringValue("saved")}}
			org.Objects["Account"] = account
			if err = store.Save(org); err != nil {
				t.Fatal(err)
			}
			if err = store.Close(); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "deleted":
				if err = os.Remove(path); err != nil {
					t.Fatal(err)
				}
			case "inactive":
				writeTestFile(t, path, strings.Replace(string(source), "<status>Active</status>", "<status>Draft</status>", 1))
			case "renamed":
				if err = os.Rename(path, filepath.Join(dir, "Renamed.flow-meta.xml")); err != nil {
					t.Fatal(err)
				}
			case "moved":
				writeTestFile(t, path, strings.Replace(string(source), "<object>Account</object>", "<object>Contact</object>", 1))
			case "invalidated":
				writeTestFile(t, path, "<Flow")
			}
			for _, pass := range []string{"cold", "warm"} {
				t.Run(pass, func(t *testing.T) {
					s, current, err := openDBStore(db, root)
					if err != nil {
						if mode == "invalidated" {
							return
						}
						t.Fatal(err)
					}
					defer s.Close()
					count := func(object, name string) int {
						n := 0
						for _, rule := range current.Objects[object].Definition.FlowRules {
							if strings.EqualFold(rule.Name, name) {
								n++
							}
						}
						return n
					}
					if count("Account", "Unrelated") != 1 {
						t.Fatal("runtime-only rule lost")
					}
					if count("Contact", "Anchor") != 1 {
						t.Fatal("valid project Flow lost")
					}
					if current.Objects["Account"].Records[storage.ID("001000000000001AAA")].Fields["Name"].String != "saved" {
						t.Fatal("saved row lost")
					}
					if mode != "invalidated" && count("Account", "Tracked") != 0 {
						t.Fatal("obsolete Account rule retained")
					}
					if mode == "invalidated" {
						for _, r := range current.Objects["Account"].Definition.FlowRules {
							if r.Name == "Tracked" && r.TriggerType == "RecordBeforeSave" {
								t.Fatal("invalidated rule retained")
							}
						}
					}
					if mode == "renamed" && count("Account", "Renamed") != 1 {
						t.Fatal("renamed rule missing")
					}
					if mode == "moved" && count("Contact", "Tracked") != 1 {
						t.Fatal("moved rule missing")
					}
				})
			}
		})
	}
}
