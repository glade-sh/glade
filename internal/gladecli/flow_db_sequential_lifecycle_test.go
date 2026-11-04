package gladecli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowDBSequentialSourceTransitions(t *testing.T) {
	source, err := os.ReadFile("../automation/testdata/flow_merge_format/Format_constant.flow-meta.xml")
	if err != nil {
		t.Fatal(err)
	}
	quote := strings.Replace(string(source), "<object>Account</object>", "<object>Quote</object>", 1)
	item := strings.Replace(string(source), "<object>Account</object>", "<object>CampaignMember</object>", 1)
	for _, tc := range []struct {
		mode   string
		stored bool
	}{
		{"delete_then_add", true}, {"delete_then_add", false}, {"move_then_delete", true}, {"move_then_delete", false},
	} {
		mode := tc.mode
		t.Run(fmt.Sprintf("%s/stored_%t", mode, tc.stored), func(t *testing.T) {
			root := t.TempDir()
			writeTestProject(t, root)
			path := filepath.Join(root, "force-app/main/default/flows/Tracked.flow-meta.xml")
			writeTestFile(t, path, quote)
			db := filepath.Join(root, "org.sqlite")
			store, org, err := openDBStore(db, root)
			if err != nil {
				t.Fatal(err)
			}
			q := org.Objects["Quote"]
			if len(q.Definition.FlowRules) == 0 {
				t.Fatal("fixture did not attach Flow")
			}
			id := storage.ID("0Q0000000000001AAA")
			if tc.stored {
				q.Records[id] = storage.Record{Object: "Quote", ID: id, Fields: map[string]storage.Value{"Name": storage.StringValue("saved")}}
			}
			org.Objects["Quote"] = q
			user := org.Objects["User"]
			user.Definition.FlowRules = append(user.Definition.FlowRules, storage.FlowRule{Name: "RuntimeOnly", Active: true})
			org.Objects["User"] = user
			if err = store.Save(org); err != nil {
				t.Fatal(err)
			}
			if err = store.Close(); err != nil {
				t.Fatal(err)
			}
			_, initialBinding, _, err := projectOrgAndDBBinding(root)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "delete_then_add" {
				if err = os.Remove(path); err != nil {
					t.Fatal(err)
				}
			} else {
				writeTestFile(t, path, item)
			}
			_, firstBinding, _, err := projectOrgAndDBBinding(root)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "move_then_delete" && firstBinding.SchemaFingerprint == initialBinding.SchemaFingerprint {
				t.Fatal("fixture first transition did not change schema fingerprint")
			}
			store, afterFirst, err := openDBStore(db, root)
			if err != nil {
				t.Fatalf("first transition: %v", err)
			}
			foundFirst := false
			for _, r := range afterFirst.Objects["User"].Definition.FlowRules {
				if r.Name == "RuntimeOnly" {
					foundFirst = true
				}
			}
			if !foundFirst {
				t.Fatal("runtime-only Flow lost on first transition")
			}
			if err = store.Close(); err != nil {
				t.Fatal(err)
			}
			if mode == "delete_then_add" {
				writeTestFile(t, path, item)
			} else {
				if err = os.Remove(path); err != nil {
					t.Fatal(err)
				}
			}
			_, secondBinding, _, err := projectOrgAndDBBinding(root)
			if err != nil {
				t.Fatal(err)
			}
			if secondBinding.SchemaFingerprint == firstBinding.SchemaFingerprint {
				t.Fatal("fixture second transition did not change schema fingerprint")
			}
			for _, pass := range []string{"cold", "warm"} {
				t.Run(pass, func(t *testing.T) {
					s, current, err := openDBStore(db, root)
					if err != nil {
						t.Fatal(err)
					}
					defer s.Close()
					if tc.stored && current.Objects["Quote"].Records[id].Fields["Name"].String != "saved" {
						t.Fatal("saved Quote lost")
					}
					found := false
					for _, r := range current.Objects["User"].Definition.FlowRules {
						if r.Name == "RuntimeOnly" {
							found = true
						}
					}
					if !found {
						t.Fatal("runtime-only Flow lost")
					}
				})
			}
		})
	}
}
