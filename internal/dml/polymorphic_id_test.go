package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestTaskCustomParentEquivalentID(t *testing.T) {
	for _, storedID := range []storage.ID{"a01000000000001", "a01000000000001AAA"} {
		for _, deleted := range []bool{false, true} {
			name := string(storedID)
			if deleted {
				name += "/deleted"
			}
			t.Run(name, func(t *testing.T) {
				org := testOrg()
				storage.EnsureStandardObject(&org, "Task")
				parent := storage.Record{ID: storedID, Object: "OwnedParent__c", Fields: map[string]storage.Value{"Name": storage.StringValue("parent")}}
				parent.System.IsDeleted = deleted
				org.Objects[parent.Object] = storage.ObjectState{
					Definition: storage.ObjectDefinition{APIName: parent.Object, KeyPrefix: "a01", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}}},
					Records:    map[storage.ID]storage.Record{storedID: parent},
				}
				reference := storage.ID("a01000000000001AAA")
				if len(storedID) == 18 {
					reference = "a01000000000001"
				}
				engine := NewEngine(&org)
				result := engine.Insert([]storage.Record{{Object: "Task", Fields: map[string]storage.Value{
					"Subject": storage.StringValue("reference proof"), "WhatId": storage.IDValue(reference),
					"Status": storage.StringValue("Not Started"), "Priority": storage.StringValue("Normal"),
				}}})
				if result[0].Success == deleted {
					t.Fatalf("Task insert = %#v; parent deleted=%v", result, deleted)
				}
				if result[0].Success {
					task := org.Objects["Task"].Records[result[0].ID]
					value, _ := task.GetField("WhatId")
					if !storage.IDsEqual(idFromStorageValue(value), storedID) {
						t.Fatalf("persisted reference = %#v; want %s", value, storedID)
					}
				}
			})
		}
	}
}
