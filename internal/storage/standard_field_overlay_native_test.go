package storage

import (
	"encoding/json"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestStandardFieldCompatibilityOverlayNativeDescribeMetadata(t *testing.T) {
	for _, api := range []string{"62", "67"} {
		path := "../apextest/testdata/conformance/standard_field_overlay/native/api" + api + "/org.tsv"
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		seen := 0
		for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
			columns := strings.SplitN(line, "\t", 4)
			if len(columns) != 4 || !strings.HasPrefix(columns[2], "describe") {
				continue
			}
			var native map[string]struct {
				Type, RelationshipName                                              string
				Length, Precision, Scale                                            int
				Nillable, Createable, Updateable, Permissionable, DefaultedOnCreate bool
				ReferenceTo, Picklist                                               []string
			}
			if err := json.Unmarshal([]byte(columns[3]), &native); err != nil || len(native) == 0 {
				t.Fatalf("incomplete native describe: %v", err)
			}
			seen++
			for name, want := range native {
				object, fieldName, _ := strings.Cut(name, ".")
				for _, route := range []string{"definition", "emptyOverlay", "featureOverlay", "repairEnsureMissing", "repairRefreshMissing", "repairEnsureAny", "repairRefreshAny", "repairEnsureDisplay", "repairRefreshDisplay"} {
					t.Run(api+"/"+columns[1]+"/"+route+"/"+name, func(t *testing.T) {
						definition := ObjectDefinition{APIName: object}
						if strings.HasPrefix(route, "repair") {
							definition, _ = StandardObjectDefinition(object)
							delete(definition.Fields, fieldName)
							if strings.HasSuffix(route, "Any") {
								definition.Fields[fieldName] = Field{APIName: fieldName, Type: FieldAny, DisplayType: "ANY"}
							}
							if strings.HasSuffix(route, "Display") {
								fieldType := FieldType(want.Type)
								if want.Type == "PHONE" {
									fieldType = FieldString
								}
								definition.Fields[fieldName] = Field{APIName: fieldName, Type: fieldType}
							}
							if !StandardObjectFieldsNeedWrite(definition) {
								t.Fatal("current overlay did not request native field repair: " + name)
							}
							if strings.Contains(route, "Refresh") {
								if !RefreshStandardObjectFieldOverlay(&definition) {
									t.Fatal("native field refresh was skipped")
								}
							} else {
								EnsureStandardObjectFields(&definition)
							}
						} else if route == "definition" {
							var ok bool
							definition, ok = StandardObjectDefinition(object)
							if !ok {
								t.Fatal("missing standard object: " + object)
							}
						} else if route == "featureOverlay" {
							EnsureStandardObjectFieldsForFeatures(&definition, []string{"MultiCurrency", "PersonAccounts", "StateAndCountryPicklist"})
						} else {
							EnsureStandardObjectFields(&definition)
						}
						field, ok := definition.Fields[fieldName]
						if !ok {
							t.Fatal("native field absent: " + name)
						}
						if field.DisplayType != want.Type || field.Length != want.Length || field.Precision != want.Precision || field.Scale != want.Scale || field.RelationshipName != want.RelationshipName || strings.Join(field.ReferenceTo, ",") != strings.Join(want.ReferenceTo, ",") {
							t.Errorf("native shape=%+v; Glade=%+v", want, field)
						}
						for flag, pair := range map[string]struct {
							got  *bool
							want bool
						}{
							"nillable": {field.Nillable, want.Nillable}, "createable": {field.Createable, want.Createable}, "updateable": {field.Updateable, want.Updateable}, "permissionable": {field.Permissionable, want.Permissionable}, "defaultedOnCreate": {field.DefaultedOnCreate, want.DefaultedOnCreate},
						} {
							if pair.got == nil || *pair.got != pair.want {
								t.Errorf("native %s=%v; Glade=%v", flag, pair.want, pair.got)
							}
						}
						var picks []string
						for _, value := range field.PicklistValues {
							picks = append(picks, value.Value+":"+strconv.FormatBool(value.Active)+":"+strconv.FormatBool(value.Default))
						}
						if strings.Join(picks, ";") != strings.Join(want.Picklist, ";") {
							t.Errorf("native picklist=%v; Glade=%v", want.Picklist, picks)
						}
						before := definition.Clone()
						EnsureStandardObjectFields(&definition)
						if route != "featureOverlay" && !reflect.DeepEqual(before, definition) {
							t.Error("repeated overlay changed the definition")
						}
					})
				}
			}
		}
		if seen != 8 {
			t.Fatalf("missing anonymous/isTest describe twins: %d", seen)
		}
	}
}
