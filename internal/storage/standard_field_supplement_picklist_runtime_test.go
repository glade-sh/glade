package storage

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

// Compare repaired runtime fields to the owned granted P0 describe, not to
// the catalog allowlist; that allowlist is not a complete native value set.
func TestStandardFieldSupplementNativePicklistsSurviveRepair(t *testing.T) {
	f, err := os.Open("../apextest/testdata/conformance/standard_describe_profiles/P0/api62/granted/describes.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var native struct {
		Describes map[string]standardDescribeObject `json:"describes"`
	}
	if err := json.NewDecoder(z).Decode(&native); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ object, field string }{{"Account", "CleanStatus"}, {"Account", "Ownership"}, {"Lead", "CleanStatus"}, {"Lead", "GenderIdentity"}, {"Lead", "Pronouns"}} {
		t.Run(tc.object+"/"+tc.field, func(t *testing.T) {
			var want []string
			for _, f := range native.Describes[tc.object].Fields {
				if f.Name == tc.field {
					for _, p := range f.PicklistValues {
						want = append(want, p.Value)
					}
				}
			}
			if len(want) == 0 {
				t.Fatal("missing native picklist")
			}
			def, ok := StandardObjectDefinition(tc.object)
			if !ok {
				t.Fatal("missing object")
			}
			field, ok := def.Fields[tc.field]
			if !ok {
				t.Fatal("missing field")
			}
			field.Type = FieldAny
			field.DisplayType = "ANY"
			field.PicklistValues = nil
			def.Fields[tc.field] = field
			EnsureStandardObjectFields(&def)
			var got []string
			for _, p := range def.Fields[tc.field].PicklistValues {
				got = append(got, p.Value)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("repaired values = %v, native values = %v", got, want)
			}
		})
	}
}

// Every field governed by the picklist repair policy is compared with its
// owned native describe, including fields added to the policy in the future.
func TestStandardPolicyPicklistsRepairAllNativeFields(t *testing.T) {
	f, err := os.Open("../apextest/testdata/conformance/standard_describe_profiles/P0/api62/granted/describes.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var native struct {
		Describes map[string]standardDescribeObject `json:"describes"`
	}
	if err := json.NewDecoder(z).Decode(&native); err != nil {
		t.Fatal(err)
	}
	count := 0
	for key, policy := range standardFieldPolicies {
		object := ""
		for name := range native.Describes {
			if strings.EqualFold(name, key) {
				object = name
				break
			}
		}
		for _, expected := range policy.picklistFields {
			name := expected.APIName
			t.Run(object+"/"+name, func(t *testing.T) {
				var want []string
				for _, f := range native.Describes[object].Fields {
					if f.Name == name {
						for _, p := range f.PicklistValues {
							want = append(want, p.Value)
						}
					}
				}
				if len(want) == 0 {
					t.Fatal("missing native list")
				}
				def, ok := StandardObjectDefinition(object)
				if !ok {
					t.Fatal("missing standard object")
				}
				field, ok := def.Fields[name]
				if !ok {
					t.Fatal("missing standard field")
				}
				field.PicklistValues = nil
				field.PicklistValuesConfigured = false
				def.Fields[name] = field
				EnsureStandardObjectFields(&def)
				var got []string
				for _, p := range def.Fields[name].PicklistValues {
					got = append(got, p.Value)
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("repaired=%v native=%v", got, want)
				}
			})
			count++
		}
	}
	if count != 8 {
		t.Fatalf("review policy coverage when field count changes: got %d, want 8", count)
	}
}
