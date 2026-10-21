package vm

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func lookupOverflowOrg(namespace string) storage.OrgState {
	org := storage.NewOrgState()
	org.Namespace = namespace
	editable, readOnly := true, false
	fields := map[string]storage.Field{
		"Id":            {APIName: "Id", Type: storage.FieldID},
		"Name":          {APIName: "Name", Type: storage.FieldString},
		"AnnualRevenue": {APIName: "AnnualRevenue", Type: storage.FieldDecimal, Scale: 2},
	}
	for _, name := range []string{"OwnerId", "ParentId", "RecordTypeId"} {
		fields[name] = storage.Field{APIName: name, Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: strings.TrimSuffix(name, "Id"), Updateable: &editable}
	}
	fields["CreatedById"] = storage.Field{APIName: "CreatedById", Type: storage.FieldReference, ReferenceTo: []string{"User"}, RelationshipName: "CreatedBy", Updateable: &readOnly}
	values := map[string]storage.Value{"Name": storage.StringValue("Lookup owner"), "AnnualRevenue": storage.DecimalValue("12")}
	for _, base := range []string{"AlphaBridge", "BetaBridge", "GammaBridge", "ZetaBridge"} {
		name, relation := base+"__c", base+"__r"
		if namespace != "" {
			name = namespace + "__" + name
			relation = namespace + "__" + relation
		}
		fields[name] = storage.Field{APIName: name, Type: storage.FieldReference, ReferenceTo: []string{"Contact"}, RelationshipName: relation, Updateable: &editable}
		values[name] = storage.IDValue("003000000000001")
	}
	org.Objects["Account"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Account", KeyPrefix: "001", Fields: fields}, Records: map[storage.ID]storage.Record{
		"001000000000001": {ID: "001000000000001", Object: "Account", Fields: values},
	}}
	org.Objects["Contact"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Contact", KeyPrefix: "003", Fields: map[string]storage.Field{
		"Id": {APIName: "Id", Type: storage.FieldID}, "Name": {APIName: "Name", Type: storage.FieldString},
	}}, Records: map[storage.ID]storage.Record{
		"003000000000001": {ID: "003000000000001", Object: "Contact", Fields: map[string]storage.Value{"Name": storage.StringValue("Selected parent")}},
	}}
	return org
}

func executeLookupOverflow(t *testing.T, namespace, api string, isTest bool, source string) *VM {
	t.Helper()
	program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
	if err != nil {
		t.Fatal(err)
	}
	org := lookupOverflowOrg(namespace)
	machine := New(nil)
	machine.SetOrg(&org)
	if isTest {
		machine.EnableTestContext()
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatalf("%v; eligible=%v", err, machine.Globals["eligible"])
	}
	return machine
}

// Fixing the traversal explicitly separates field projection from an unknown
// native describe order. Selected values and unrelated omissions are controls.
func TestLookupOverflowFixedTraversalProjection(t *testing.T) {
	for _, namespace := range []string{"", "MELODY"} {
		for _, api := range []string{"46.0", "62.0", "67.0", "68.0"} {
			for _, isTest := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/test=%t", namespace, api, isTest), func(t *testing.T) {
					executeLookupOverflow(t, namespace, api, isTest, `
Map<String,Schema.SObjectField> fields=Account.SObjectType.getDescribe().fields.getMap();
List<String> selections=new List<String>{'Id','AnnualRevenue'};
for(String name:new List<String>{'AlphaBridge__c','BetaBridge__c','GammaBridge__c'}){
 Schema.DescribeFieldResult facet=fields.get(name).getDescribe();
 selections.add(facet.getName());selections.add(facet.getRelationshipName()+'.Name');
}
Id owner='001000000000001';
Account main=Database.query('SELECT '+String.join(selections,',')+' FROM Account WHERE Id=:owner');
Schema.DescribeFieldResult overflowFacet=Account.ZetaBridge__c.getDescribe();
Account extra=Database.query('SELECT Id,'+overflowFacet.getName()+','+overflowFacet.getRelationshipName()+'.Name FROM Account WHERE Id=:owner');
Boolean omitted=false;try{Object v=main.get(Account.ZetaBridge__c);}catch(SObjectException e){omitted=true;}
System.assert(omitted,'Separate query must not silently widen the main projection');
main.AnnualRevenue+=1;
omitted=false;try{Object v=main.get('ZetaBridge__c');}catch(SObjectException e){omitted=true;}System.assert(omitted);
System.assertEquals('003000000000001',extra.get(Account.ZetaBridge__c));
System.assertEquals('Selected parent',extra.getSObject(overflowFacet.getRelationshipName()).get('Name'));
main.put(Account.ZetaBridge__c,extra.get(Account.ZetaBridge__c));
main.putSObject(overflowFacet.getRelationshipName(),extra.getSObject(overflowFacet.getRelationshipName()));
System.assertEquals('003000000000001',main.get('ZetaBridge__c'));
System.assertEquals('Selected parent',main.getSObject(overflowFacet.getRelationshipName()).get('Name'));
System.assertEquals('Selected parent',main.getSObject(Account.AlphaBridge__c.getDescribe().getRelationshipName()).get('Name'));
System.assertEquals(13,main.AnnualRevenue);
`)
				})
			}
		}
	}
}

// PENDING-ROWS: this tests the candidate hypothesis that qualification preserves
// the local-key traversal policy. No native ordinal is asserted as established.
func TestLookupOverflowOwnedDescribeTraversalCandidate(t *testing.T) {
	for _, namespace := range []string{"", "MELODY", "melody"} {
		for _, api := range []string{"46.0", "62.0", "67.0", "68.0"} {
			for _, isTest := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/test=%t", namespace, api, isTest), func(t *testing.T) {
					executeLookupOverflow(t, namespace, api, isTest, `
Map<String,Schema.SObjectField> fields=Account.SObjectType.getDescribe().fields.getMap();
List<String> eligible=new List<String>();
List<Schema.SObjectField> overflow=new List<Schema.SObjectField>();
Integer ordinal=0;
for(String key:new List<String>(new Set<String>(fields.keySet()))){
 Schema.SObjectField token=fields.get(key);Schema.DescribeFieldResult facet=token.getDescribe();
 if(facet.getType()!=Schema.DisplayType.REFERENCE||!facet.isUpdateable())continue;
 eligible.add(facet.getLocalName());
 if(ordinal>=4)overflow.add(token);
 ordinal++;
}
System.assertEquals(8,eligible.size());
System.assert(overflow.contains(Account.ZetaBridge__c),'Candidate places the last local lookup beyond four slots');
List<String> valueNames=new List<String>();
for(Schema.SObjectField token:fields.values()){
 Schema.DescribeFieldResult facet=token.getDescribe();
 if(facet.getType()==Schema.DisplayType.REFERENCE&&facet.isUpdateable())valueNames.add(facet.getLocalName());
}
System.assertEquals(eligible,valueNames,'Key and value traversal stay aligned');
`)
				})
			}
		}
	}
}

func TestLookupOverflowExplicitLimitControls(t *testing.T) {
	for _, capacity := range []int{0, 3, 4, 8} {
		t.Run(fmt.Sprint(capacity), func(t *testing.T) {
			executeLookupOverflow(t, "MELODY", "46.0", true, fmt.Sprintf(`
List<Schema.SObjectField> references=new List<Schema.SObjectField>{Account.AlphaBridge__c,Account.BetaBridge__c,Account.GammaBridge__c,Account.ZetaBridge__c};
Set<Schema.SObjectField> main=new Set<Schema.SObjectField>();Set<Schema.SObjectField> extra=new Set<Schema.SObjectField>();
for(Integer i=0;i<references.size();i++){if(i<%d)main.add(references[i]);else extra.add(references[i]);}
System.assertEquals(4,main.size()+extra.size());
System.assertEquals(%t,extra.contains(Account.ZetaBridge__c));
Map<String,Integer> ordinary=new Map<String,Integer>();ordinary.put('Zeta',1);ordinary.put('Alpha',2);
System.assertEquals(new List<String>{'Zeta','Alpha'},new List<String>(ordinary.keySet()));
`, capacity, capacity < 4))
		})
	}
}

func TestLookupOverflowDescribeCandidateScope(t *testing.T) {
	editable := true
	for _, tc := range []struct {
		name, object, namespace string
		fields                  []storage.Field
	}{
		{"plain", "Account", "", []storage.Field{{APIName: "ZetaBridge__c", Type: storage.FieldReference, Updateable: &editable}}},
		{"different object", "Contact", "MELODY", []storage.Field{{APIName: "MELODY__ZetaBridge__c", Type: storage.FieldReference, Updateable: &editable}}},
		{"no declared reference", "Account", "MELODY", []storage.Field{{APIName: "MELODY__ZetaBridge__c", Type: storage.FieldString}}},
		{"foreign owner", "Account", "MELODY", []storage.Field{{APIName: "OTHER__ZetaBridge__c", Type: storage.FieldReference, Updateable: &editable}}},
		{"mixed owners", "Account", "MELODY", []storage.Field{{APIName: "MELODY__ZetaBridge__c", Type: storage.FieldReference, Updateable: &editable}, {APIName: "OTHER__AlphaBridge__c", Type: storage.FieldReference, Updateable: &editable}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			definition := storage.ObjectDefinition{APIName: tc.object, Fields: map[string]storage.Field{"OwnerId": {APIName: "OwnerId", Type: storage.FieldReference}}}
			names := []string{"OwnerId"}
			for _, field := range tc.fields {
				definition.Fields[field.APIName] = field
				names = append(names, field.APIName)
			}
			expected := append([]string(nil), names...)
			sort.Strings(expected)
			machine := New(nil)
			org := storage.NewOrgState()
			org.Namespace = tc.namespace
			machine.SetOrg(&org)
			machine.sortDescribeFieldNames(tc.object, definition, names)
			if !slices.Equal(names, expected) {
				t.Fatalf("scope control reordered fields: %v; want %v", names, expected)
			}
		})
	}
}

func TestLookupOverflowDescribeCandidateRawKeyScope(t *testing.T) {
	for _, tc := range []struct{ name, key, api string }{
		{"blank foreign API name", "OTHER__AlphaBridge__c", ""},
		{"conflicting ownership", "OTHER__AlphaBridge__c", "MELODY__AlphaBridge__c"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			names := []string{"OwnerId", "MELODY__ZetaBridge__c", tc.key}
			expected := append([]string(nil), names...)
			sort.Strings(expected)
			definition := storage.ObjectDefinition{APIName: "Account", Fields: map[string]storage.Field{
				"OwnerId":               {APIName: "OwnerId", Type: storage.FieldReference},
				"MELODY__ZetaBridge__c": {APIName: "MELODY__ZetaBridge__c", Type: storage.FieldReference},
				tc.key:                  {APIName: tc.api, Type: storage.FieldReference},
			}}
			machine := New(nil)
			org := storage.NewOrgState()
			org.Namespace = "MELODY"
			machine.SetOrg(&org)
			machine.sortDescribeFieldNames("Account", definition, names)
			if !slices.Equal(names, expected) {
				t.Fatalf("mixed raw ownership reordered fields: %v; want %v", names, expected)
			}
		})
	}
}
