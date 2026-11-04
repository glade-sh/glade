package gladecli

import (
	"github.com/glade-sh/glade/internal/storage"
	"os"
	"strings"

	"github.com/glade-sh/glade/internal/dml"
	"path/filepath"
	"testing"
)

func TestFlowMergeDBRefreshesLegacyMetadata(t *testing.T) {
	root := t.TempDir()
	writeTestProject(t, root)
	source, err := os.ReadFile("../automation/testdata/flow_merge_format/Format_constant.flow-meta.xml")
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "force-app/main/default/flows/Format_constant.flow-meta.xml"), string(source))
	db := filepath.Join(root, "org.sqlite")
	store, org, err := openDBStore(db, root)
	if err != nil {
		t.Fatal(err)
	}
	object := org.Objects["Account"]
	if len(object.Definition.FlowRules) == 0 {
		t.Fatal("fixture did not load flow")
	}
	object.Records[storage.ID("001000000000001AAA")] = storage.Record{Object: "Account", ID: storage.ID("001000000000001AAA"), Fields: map[string]storage.Value{"Name": storage.StringValue("saved")}}
	// Simulate a persisted Flow compiled by the earlier runtime with the same schema.
	for i := range object.Definition.FlowRules {
		for j := range object.Definition.FlowRules[i].Steps {
			for k := range object.Definition.FlowRules[i].Steps[j].CustomError.Messages {
				object.Definition.FlowRules[i].Steps[j].CustomError.Messages[k].NumberConstants = nil
			}
		}
		for j := range object.Definition.FlowRules[i].CustomErrors {
			for k := range object.Definition.FlowRules[i].CustomErrors[j].Messages {
				object.Definition.FlowRules[i].CustomErrors[j].Messages[k].NumberConstants = nil
			}
		}
	}
	org.Objects["Account"] = object
	if err = store.Save(org); err != nil {
		t.Fatal(err)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	for _, pass := range []string{"refresh", "warm"} {
		t.Run(pass, func(t *testing.T) {
			s, current, err := openDBStore(db, root)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			a := current.Objects["Account"]
			if a.Records[storage.ID("001000000000001AAA")].Fields["Name"].String != "saved" {
				t.Fatal("saved record lost")
			}
			found := false
			for _, r := range a.Definition.FlowRules {
				for _, e := range r.CustomErrors {
					for _, m := range e.Messages {
						if m.NumberConstants["large"] == "1234.50" {
							found = true
						}
					}
				}
			}
			if !found {
				t.Fatal("legacy Flow numeric metadata was retained")
			}
			users := current.Objects["User"]
			if users.Records == nil {
				users.Records = make(map[storage.ID]storage.Record)
			}
			user := users.Records[storage.DefaultUserID]
			if user.Fields == nil {
				user.Fields = make(map[string]storage.Value)
			}
			user.Fields["LocaleSidKey"] = storage.StringValue("en_US")
			users.Records[storage.DefaultUserID] = user
			current.Objects["User"] = users
			engine := dml.NewEngine(&current)
			result := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("FORMAT_CONSTANT")}}})[0]
			if result.Success || !strings.Contains(result.Error, "constant=1,234.5|negative=-1,234.5|boundary=1,000|below=999.5") {
				t.Fatalf("reopened Flow result: %+v", result)
			}
		})
	}
}
