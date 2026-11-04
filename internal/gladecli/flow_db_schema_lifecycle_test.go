package gladecli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowDBLastSourceDoesNotChangeSchema(t *testing.T) {
	source, err := os.ReadFile("../automation/testdata/flow_merge_format/Format_constant.flow-meta.xml")
	if err != nil {
		t.Fatal(err)
	}
	source = []byte(strings.Replace(string(source), "<object>Account</object>", "<object>Quote</object>", 1))
	for _, mode := range []string{"delete", "move", "add"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			writeTestProject(t, root)
			path := filepath.Join(root, "force-app/main/default/flows/Tracked.flow-meta.xml")
			if mode != "add" {
				writeTestFile(t, path, string(source))
			}
			db := filepath.Join(root, "org.sqlite")
			store, org, err := openDBStore(db, root)
			if err != nil {
				t.Fatal(err)
			}
			if mode != "add" {
				object, ok := org.Objects["Quote"]
				if !ok || len(object.Definition.FlowRules) == 0 {
					t.Fatal("fixture must load the Quote Flow")
				}
			}
			user := org.Objects["User"]
			user.Definition.FlowRules = append(user.Definition.FlowRules, storage.FlowRule{Name: "RuntimeOnly", Active: true})
			org.Objects["User"] = user
			if mode != "add" {
				account := org.Objects["Quote"]
				account.Records[storage.ID("0Q0000000000001AAA")] = storage.Record{Object: "Quote", ID: storage.ID("0Q0000000000001AAA"), Fields: map[string]storage.Value{"Name": storage.StringValue("saved")}}
				org.Objects["Quote"] = account
			}
			if err = store.Save(org); err != nil {
				t.Fatal(err)
			}
			if err = store.Close(); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "delete":
				if err = os.Remove(path); err != nil {
					t.Fatal(err)
				}
			case "move":
				writeTestFile(t, path, strings.Replace(string(source), "<object>Quote</object>", "<object>OpportunityLineItem</object>", 1))
			case "add":
				writeTestFile(t, path, string(source))
			}
			for _, pass := range []string{"cold", "warm"} {
				t.Run(pass, func(t *testing.T) {
					s, current, err := openDBStore(db, root)
					if err != nil {
						t.Fatal(err)
					}
					defer s.Close()
					found := false
					for _, rule := range current.Objects["User"].Definition.FlowRules {
						if rule.Name == "RuntimeOnly" {
							found = true
						}
					}
					if !found {
						t.Fatal("runtime-only flow lost after source lifecycle change")
					}
					if mode != "add" && current.Objects["Quote"].Records[storage.ID("0Q0000000000001AAA")].Fields["Name"].String != "saved" {
						t.Fatal("saved Quote lost")
					}
					target := "Quote"
					if mode == "move" {
						target = "OpportunityLineItem"
					}
					if mode != "delete" {
						found = false
						for _, rule := range current.Objects[target].Definition.FlowRules {
							if rule.Name == "Tracked" {
								found = true
							}
						}
						if !found {
							t.Fatalf("project Flow missing on %s", target)
						}
					}
				})
			}
		})
	}
}
