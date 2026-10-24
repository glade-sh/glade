package storage_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sobject"
	"github.com/glade-sh/glade/internal/storage"
)

func TestPersonAccountCustomFieldsNativeSchemaRoutes(t *testing.T) {
	const fixture = "../apextest/testdata/conformance/person_account_custom_fields/"
	raw, err := os.ReadFile(fixture + "cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases struct{ Files map[string]string }
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("sfdx-project.json", `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"62.0"}`)
	for name, body := range cases.Files {
		write(name, body)
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	registry := sobject.BuildDescribeRegistry(s)
	raw, err = os.ReadFile(fixture + "org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
		row := strings.SplitN(line, "\t", 4)
		if len(row) != 4 {
			t.Fatal(line)
		}
		if !strings.HasPrefix(row[2], "describe_") {
			continue
		}
		checked++
		t.Run(strings.Join(row[:3], "/"), func(t *testing.T) {
			var want []any
			if err := json.Unmarshal([]byte(row[3]), &want); err != nil || len(want) != 12 {
				t.Fatalf("invalid native row: %s", line)
			}
			org := storage.NewOrgState()
			org.APIVersion = row[0]
			for name, describe := range registry.Objects {
				org.Objects[name] = storage.ObjectState{Definition: sobject.ToObjectDefinition(describe)}
			}
			contactBefore := org.Objects["Contact"].Definition.Clone()
			storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
			name := want[0].(string)
			field, ok := org.Objects["Account"].Definition.Fields[name]
			if !ok {
				t.Fatalf("native Account.%s missing from PersonAccounts schema", name)
			}
			source := org.Objects["Contact"].Definition.Fields[strings.TrimSuffix(name, "__pc")+"__c"]
			// Describe defaults are checked by runtime conformance; storage keeps
			// the original metadata length, including an omitted length.
			if field.Type != source.Type || field.DisplayType != want[1] || field.Label != want[2] || field.Length != source.Length || float64(field.Precision) != want[4] || float64(field.Scale) != want[5] {
				t.Errorf("native %v; mirrored field %+v", want, field)
			}
			if want[11] != nil && field.RelationshipName != want[11] {
				t.Errorf("native relationship %v; mirrored %q", want[11], field.RelationshipName)
			}
			if _, ok := org.Objects["Account"].Definition.Fields["Absent__pc"]; ok {
				t.Fatal("fabricated an absent Contact field")
			}
			before := org.Objects["Account"].Definition.Clone()
			storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
			if !reflect.DeepEqual(before.Fields, org.Objects["Account"].Definition.Fields) {
				t.Fatal("repeat feature application changed mirrored fields")
			}
			if want[11] != nil {
				count := 0
				for _, relation := range org.Objects["Account"].Definition.Relations {
					if relation.Field == name && relation.ParentRelationship == want[11] {
						count++
					}
				}
				if count != 1 {
					t.Fatalf("mirrored lookup relationships=%d, want 1", count)
				}
			}
			for key, field := range contactBefore.Fields {
				if !strings.HasSuffix(strings.ToLower(key), "__c") {
					continue
				}
				if !reflect.DeepEqual(field, org.Objects["Contact"].Definition.Fields[key]) {
					t.Fatalf("mirroring changed Contact.%s", key)
				}
			}
			parentRelations := org.Objects["Account"].Definition.Clone().Relations
			snapshot := func(value any) string {
				t.Helper()
				body, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				return string(body)
			}
			parentFields := snapshot(org.Objects["Account"].Definition.Fields)
			sourceField := snapshot(source)
			clone := org.CloneRuntimeFrozenDefinition()
			storage.ApplyOrgShape(&clone, []string{"PersonAccounts"})
			for _, relation := range clone.Objects["Account"].Definition.Relations {
				if relation.Field == name && len(relation.ParentObjects) > 0 {
					relation.ParentObjects[0] = "ChangedTarget"
				}
			}
			private := clone.Objects["Account"].Definition.Fields[name]
			if len(private.ReferenceTo) > 0 {
				private.ReferenceTo[0] = "ChangedTarget"
			}
			if len(private.PicklistValues) > 0 {
				private.PicklistValues[0].Label = "Changed label"
			}
			if private.Nillable != nil {
				*private.Nillable = !*private.Nillable
			}
			if parentFields != snapshot(org.Objects["Account"].Definition.Fields) {
				t.Fatal("clone feature application mutated parent")
			}
			if !reflect.DeepEqual(parentRelations, org.Objects["Account"].Definition.Relations) {
				t.Fatal("clone mutated parent lookup relationships")
			}
			if sourceField != snapshot(org.Objects["Contact"].Definition.Fields[strings.TrimSuffix(name, "__pc")+"__c"]) {
				t.Fatal("mutating a mirror changed Contact metadata")
			}
		})
	}
	if checked != 40 {
		t.Fatalf("native describe rows=%d, want 40", checked)
	}
}
