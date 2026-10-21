package vm

import "testing"

// Compatibility controls protect existing lookup behavior outside the native
// Id-less custom-record probes. They do not claim additional native answers.
func TestSchemaDefaultMapLookupCompatibilityExclusions(t *testing.T) {
	for _, control := range []string{"id", "setting", "noDefault", "class"} {
		t.Run(control, func(t *testing.T) {
			org := schemaCheckboxEqualityTestOrg()
			object := org.Objects["LyricTile__c"]
			if control == "setting" {
				object.Definition.Metadata = map[string]string{"kind": "customSetting"}
			}
			if control == "noDefault" {
				column := object.Definition.Fields["TempoZero__c"]
				column.DefaultValue = ""
				object.Definition.Fields["TempoZero__c"] = column
			}
			org.Objects["LyricTile__c"] = object
			machine := New(nil)
			machine.SetOrg(&org)
			stored, key := Object("LyricTile__c"), Object("LyricTile__c")
			stored.Fields["TempoZero__c"], key.Fields["TempoZero__c"] = Decimal(0), Decimal(0)
			markExplicitSObjectField(&key, "TempoZero__c")
			if control == "id" {
				stored.Fields["Id"], key.Fields["Id"] = platformScalar("Id", "a00000000000001"), platformScalar("Id", "a00000000000001")
			}
			mapType := "Map<SObject,String>"
			if control == "class" {
				stored.classInstance, key.classInstance = true, true
				mapType = "Map<Object,String>"
				hash, err := CompileAnonymous("return 7;")
				if err != nil {
					t.Fatal(err)
				}
				equals, err := CompileAnonymous("return true;")
				if err != nil {
					t.Fatal(err)
				}
				if err := machine.RegisterClass(Class{Name: "LyricTile__c", Fields: map[string]Field{"TempoZero__c": {Name: "TempoZero__c", Type: "Decimal"}}, Methods: map[string]Method{
					"hashCode": {Name: "LyricTile__c.hashCode", ClassName: "LyricTile__c", ReturnType: "Integer", Program: hash},
					"equals":   {Name: "LyricTile__c.equals", ClassName: "LyricTile__c", ReturnType: "Boolean", Params: []Param{{Name: "other", Type: "Object"}}, Program: equals},
				}}); err != nil {
					t.Fatal(err)
				}
			}
			if stored.Equal(key) {
				t.Fatal("control lacks distinct record presence")
			}
			encoded := machine.mapKey(stored)
			if encoded != machine.mapKey(key) {
				t.Fatal("control lacks physical key collision")
			}
			keyed := Map()
			keyed.Type = mapType
			machine.Globals["keyed"] = keyed
			if _, handled, err := machine.callMapValueMember("keyed", keyed, "put", []Value{stored, String("present")}, &Result{}); err != nil || !handled {
				t.Fatalf("control insertion: %t %v", handled, err)
			}
			keyed = machine.Globals["keyed"]
			for i, probe := range []Value{key, stored} {
				got, handled, err := machine.callMapValueMember("", keyed, "get", []Value{probe}, &Result{})
				if control == "id" && i == 0 {
					// The existing Id-backed path already distinguishes these
					// unequal records; the new physical-slot guard is not involved.
					if err != nil || !handled || got.Kind != ValueNull {
						t.Fatalf("Id-backed lookup changed: %v %t %v", got, handled, err)
					}
					continue
				}
				if err != nil || !handled || got.Kind != ValueString || got.Text != "present" {
					t.Fatalf("existing lookup changed: %v %t %v", got, handled, err)
				}
			}
		})
	}
}
