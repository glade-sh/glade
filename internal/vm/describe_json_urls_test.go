package vm

import (
	"encoding/json"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// These are scope controls, not native claims for unobserved describe shapes.
func TestDescribeJSONURLsPreserveUncapturedContexts(t *testing.T) {
	for _, tc := range []struct{ name, api, namespace, object, option, kind string }{
		{"earlier API", "61.0", "", "User", "DEFERRED", "Schema.DescribeSObjectResult"},
		{"intermediate API", "65.0", "", "User", "DEFERRED", "Schema.DescribeSObjectResult"},
		{"later API", "69.0", "", "User", "DEFERRED", "Schema.DescribeSObjectResult"},
		{"missing API", "", "", "User", "DEFERRED", "Schema.DescribeSObjectResult"},
		{"namespace", "67.0", "prism", "User", "DEFERRED", "Schema.DescribeSObjectResult"},
		{"other object", "67.0", "", "Contact", "DEFERRED", "Schema.DescribeSObjectResult"},
		{"full option", "67.0", "", "User", "FULL", "Schema.DescribeSObjectResult"},
		{"default option", "67.0", "", "User", "DEFAULT", "Schema.DescribeSObjectResult"},
		{"ordinary object", "67.0", "", "User", "DEFERRED", "OwnedDescriptor"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			machine := New(nil)
			machine.currentMethod = Method{APIVersion: tc.api, Namespace: tc.namespace, SourceContextBound: true}
			value := Object(tc.kind)
			value.Fields["name"] = String(tc.object)
			value.Fields["sObjectDescribeOption"] = sObjectDescribeOptionsValue(tc.option)
			if _, ok := machine.describeJSONURLs(value); ok {
				t.Fatal("uncaptured context changed")
			}
		})
	}
	machine := New(nil)
	machine.currentMethod = Method{APIVersion: "67.0", SourceContextBound: true}
	machine.Org = &storage.OrgState{Namespace: "prism"}
	value := Object("Schema.DescribeSObjectResult")
	value.Fields["name"] = String("User")
	value.Fields["sObjectDescribeOption"] = sObjectDescribeOptionsValue("DEFERRED")
	if _, ok := machine.describeJSONURLs(value); ok {
		t.Fatal("namespaced org changed")
	}
}

func TestDescribeJSONURLsPreserveExistingPropertyAndMetadata(t *testing.T) {
	machine := New(nil)
	machine.currentMethod = Method{APIVersion: "67.0", SourceContextBound: true}
	value := Object("Schema.DescribeSObjectResult")
	value.Fields["name"] = String("User")
	value.Fields["sObjectDescribeOption"] = sObjectDescribeOptionsValue("DEFERRED")
	serialize := func(suppress bool) map[string]any {
		t.Helper()
		raw, err := json.Marshal(machine.jsonFromValueForSerialize(value, suppress))
		if err != nil {
			t.Fatal(err)
		}
		var result map[string]any
		if err := json.Unmarshal(raw, &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	if _, ok := serialize(true)["urls"]; !ok {
		t.Fatal("captured property missing")
	}
	if _, ok := value.Fields["urls"]; ok {
		t.Fatal("serialization mutated metadata")
	}
	for _, existing := range []Value{Null, String("owned property")} {
		value.Fields["urls"] = existing
		if _, ok := machine.describeJSONURLs(value); ok {
			t.Fatal("existing property replaced")
		}
		for _, suppress := range []bool{false, true} {
			out := serialize(suppress)
			actual, present := out["urls"]
			if existing.Kind == ValueNull {
				if (suppress && present) || (!suppress && (!present || actual != nil)) {
					t.Fatal("existing null handling changed")
				}
			} else if actual != "owned property" {
				t.Fatal("existing property changed")
			}
		}
	}
}
