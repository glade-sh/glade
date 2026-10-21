package vm

import (
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// API 65 anonymous and named native rows retain null RecurrenceActivityId
// relationship metadata while reaching WhatId before that field.
func TestTaskDescribeReferenceTraversalNative65(t *testing.T) {
	for _, testContext := range []bool{false, true} {
		name := "anonymous"
		if testContext {
			name = "isTest"
		}
		t.Run(name, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(`
Map<String,Schema.SObjectField> fields=Task.SObjectType.getDescribe().fields.getMap();
List<String> references=new List<String>();
Set<String> capturedReferences=new Set<String>{'WhoId','WhatId','OwnerId','AccountId','CreatedById','LastModifiedById','RecurrenceActivityId'};
for(Schema.SObjectField field:fields.values())if(field.getDescribe().getType()==Schema.DisplayType.REFERENCE && capturedReferences.contains(field.getDescribe().getName()))references.add(field.getDescribe().getName());
System.assertEquals(new List<String>{'WhoId','WhatId','OwnerId','AccountId','CreatedById','LastModifiedById','RecurrenceActivityId'},references);
System.assertEquals(null,fields.get('RecurrenceActivityId').getDescribe().getRelationshipName());
String found;
for(Schema.SObjectField field:fields.values()){
 Schema.DescribeFieldResult d=field.getDescribe();
 if(d.getType()==Schema.DisplayType.REFERENCE && d.getRelationshipName().toLowerCase()=='what'){found=d.getName();break;}
}
System.assertEquals('WhatId',found);
`, CompileOptions{APIVersion: "65.0"})
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			org.APIVersion = "68.0"
			storage.EnsureStandardObject(&org, "Task")
			machine := New(nil)
			machine.SetOrg(&org)
			if testContext {
				machine.EnableTestContext()
			}
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestTaskDescribeOrderingPreservesOrdinaryMap(t *testing.T) {
	program, err := CompileAnonymousWithOptions(`
Map<String,Integer> ordinary=new Map<String,Integer>();ordinary.put('Zeta',1);ordinary.put('Alpha',2);
System.assertEquals(new List<String>{'Zeta','Alpha'},new List<String>(ordinary.keySet()));
System.assertEquals(new List<Integer>{1,2},ordinary.values());
`, CompileOptions{APIVersion: "65.0"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestTaskDescribeOrderCapturedScope(t *testing.T) {
	for _, tc := range []struct {
		name, object, api, namespace string
		want                         []string
	}{
		{"captured", "Task", "65.0", "", []string{"Id", "WhoId", "WhatId", "Alpha__c", "RecordTypeId", "Zeta__c"}},
		{"earlier API", "Task", "64.0", "", []string{"Alpha__c", "Id", "RecordTypeId", "WhatId", "WhoId", "Zeta__c"}},
		{"later API", "Task", "66.0", "", []string{"Alpha__c", "Id", "RecordTypeId", "WhatId", "WhoId", "Zeta__c"}},
		{"namespace", "Task", "65.0", "MELODY", []string{"Alpha__c", "Id", "RecordTypeId", "WhatId", "WhoId", "Zeta__c"}},
		{"other object", "Contact", "65.0", "", []string{"Alpha__c", "Id", "RecordTypeId", "WhatId", "WhoId", "Zeta__c"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := storage.NewOrgState()
			org.Namespace = tc.namespace
			org.APIVersion = "68.0"
			machine := New(nil)
			machine.SetOrg(&org)
			machine.currentMethod = Method{APIVersion: tc.api}
			names := []string{"Zeta__c", "WhatId", "Alpha__c", "Id", "RecordTypeId", "WhoId"}
			machine.sortDescribeFieldNames(tc.object, storage.ObjectDefinition{APIName: tc.object}, names)
			if !reflect.DeepEqual(names, tc.want) {
				t.Fatalf("field order=%v, want %v", names, tc.want)
			}
		})
	}
	t.Run("namespaced method", func(t *testing.T) {
		org := storage.NewOrgState()
		machine := New(nil)
		machine.SetOrg(&org)
		machine.currentMethod = Method{APIVersion: "65.0", Namespace: "MELODY"}
		names := []string{"WhoId", "WhatId", "Id"}
		machine.sortDescribeFieldNames("Task", storage.ObjectDefinition{APIName: "Task"}, names)
		if !reflect.DeepEqual(names, []string{"Id", "WhatId", "WhoId"}) {
			t.Fatalf("namespaced method changed field order: %v", names)
		}
	})
}

func TestTaskDescribeOrderCacheSourceScope(t *testing.T) {
	for _, versions := range [][]string{{"65.0", "66.0", "65.0"}, {"66.0", "65.0", "66.0"}} {
		t.Run(versions[0], func(t *testing.T) {
			org := storage.NewOrgState()
			org.APIVersion = "68.0"
			storage.EnsureStandardObject(&org, "Task")
			machine := New(nil)
			machine.SetOrg(&org)
			captured := []string{"WhoId", "WhatId", "OwnerId", "AccountId", "CreatedById", "LastModifiedById", "RecurrenceActivityId"}
			fallback := []string{"AccountId", "CreatedById", "LastModifiedById", "OwnerId", "RecurrenceActivityId", "WhatId", "WhoId"}
			for _, api := range versions {
				machine.currentMethod = Method{APIVersion: api}
				desc := machine.describeSObjectValue("Task", org.Objects["Task"].Definition)
				fields := desc.Fields["fields"].Fields["map"]
				names := []string{}
				for _, key := range fields.MapOrder {
					name := fields.MapKeys[key].Text
					for _, expected := range captured {
						if name == expected {
							names = append(names, name)
						}
					}
				}
				want := fallback
				if api == "65.0" {
					want = captured
				}
				if !reflect.DeepEqual(names, want) {
					t.Fatalf("API%s cached references=%v, want %v", api, names, want)
				}
			}
		})
	}
}
