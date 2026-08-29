package vm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestSObjectDescribeOverlayCopiesEveryMutableCollectionBranch(t *testing.T) {
	template := describeOverlayFixture(t, 32)
	overlay := overlaySObjectDescribe(template, "Widget__c", "DEFERRED")

	if got := overlay.Fields["name"].Text; got != "Widget__c" {
		t.Fatalf("name = %q, want Widget__c", got)
	}
	if got := overlay.Fields["sObjectDescribeOption"].Text; got != "DEFERRED" {
		t.Fatalf("option = %q, want DEFERRED", got)
	}
	if got := template.Fields["name"].Text; got != "pkg__Widget__c" {
		t.Fatalf("template name changed to %q", got)
	}
	if got := template.Fields["sObjectDescribeOption"].Text; got != "DEFERRED" {
		t.Fatalf("template option changed to %q", got)
	}
	deep := cloneValue(template)
	deep.Fields["name"] = String("Widget__c")
	deep.Fields["sObjectDescribeOption"] = sObjectDescribeOptionsValue("DEFERRED")
	overlayJSON, err := json.Marshal(overlay)
	if err != nil {
		t.Fatal(err)
	}
	deepJSON, err := json.Marshal(deep)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(overlayJSON, deepJSON) {
		t.Fatal("overlay differs from the former deep-clone result")
	}

	templateFields := template.Fields["fields"].Fields["map"]
	overlayFields := overlay.Fields["fields"].Fields["map"]
	if overlayFields.Ref != templateFields.Ref {
		t.Fatal("immutable overlay eagerly copied the fields map")
	}
	overlayFields = privateDescribeCollection(overlayFields)
	firstFieldKey := templateFields.MapOrder[0]
	if overlayFields.Map[firstFieldKey].Ref != templateFields.Map[firstFieldKey].Ref {
		t.Fatal("immutable field token was cloned")
	}
	delete(overlayFields.Map, firstFieldKey)
	if _, ok := templateFields.Map[firstFieldKey]; !ok {
		t.Fatal("mutating overlay fields map changed template")
	}

	templateRecordTypes := template.Fields["recordTypeInfos"]
	overlayRecordTypes := overlay.Fields["recordTypeInfos"]
	if overlayRecordTypes.Ref != templateRecordTypes.Ref {
		t.Fatal("immutable overlay eagerly copied the record type list")
	}
	overlayRecordTypes = privateDescribeCollection(overlayRecordTypes)
	overlayRecordTypes.List[0] = Null
	if templateRecordTypes.List[0].Kind == ValueNull {
		t.Fatal("mutating overlay record type list changed template")
	}

	templateFieldSet := firstDescribeFieldSet(template)
	overlayFieldSet := firstDescribeFieldSet(overlay)
	if overlayFieldSet.Ref != templateFieldSet.Ref {
		t.Fatal("immutable overlay eagerly copied the field set")
	}
	templateMembers := templateFieldSet.Fields["fields"]
	overlayMembers := overlayFieldSet.Fields["fields"]
	if overlayMembers.Ref != templateMembers.Ref {
		t.Fatal("immutable overlay eagerly copied the field set member list")
	}
	overlayMembers = privateDescribeCollection(overlayMembers)
	overlayMembers.List[0] = Null
	if templateMembers.List[0].Kind == ValueNull {
		t.Fatal("mutating overlay field set members changed template")
	}

	templateChildren := template.Fields["childRelationships"]
	overlayChildren := overlay.Fields["childRelationships"]
	if overlayChildren.Ref != templateChildren.Ref {
		t.Fatal("immutable overlay eagerly copied the child relationship list")
	}
	overlayChildren = privateDescribeCollection(overlayChildren)
	templateChild := templateChildren.List[0]
	overlayChild := overlayChildren.List[0]
	if overlayChild.Ref != templateChild.Ref {
		t.Fatal("immutable child relationship was cloned")
	}
	templateJunctions := templateChild.Fields["junctionIdListNames"]
	overlayJunctions := overlayChild.Fields["junctionIdListNames"]
	if overlayJunctions.Ref != templateJunctions.Ref {
		t.Fatal("immutable overlay eagerly copied the junction list")
	}
	overlayJunctions = privateDescribeCollection(overlayJunctions)
	overlayJunctions.List[0] = String("changed")
	if got := templateJunctions.List[0].Text; got != "LinkIds" {
		t.Fatalf("template junction name changed to %q", got)
	}
}

func TestSObjectDescribeOverlayConcurrentReadersKeepTemplateImmutable(t *testing.T) {
	template := describeOverlayFixture(t, 128)
	wantName := template.Fields["name"].Text
	wantFields := len(template.Fields["fields"].Fields["map"].Map)
	wantJunction := template.Fields["childRelationships"].List[0].Fields["junctionIdListNames"].List[0].Text

	const workers = 32
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(i int) {
			defer wg.Done()
			overlay := overlaySObjectDescribe(template, fmt.Sprintf("Local%d__c", i), "DEFERRED")
			fields := overlay.Fields["fields"]
			fieldMap := privateDescribeCollection(fields.Fields["map"])
			delete(fieldMap.Map, fieldMap.MapOrder[i%len(fieldMap.MapOrder)])
			children := privateDescribeCollection(overlay.Fields["childRelationships"])
			child := children.List[0]
			junctions := privateDescribeCollection(child.Fields["junctionIdListNames"])
			junctions.List[0] = String("changed")
			children.List[0] = child
		}(i)
	}
	wg.Wait()

	if got := template.Fields["name"].Text; got != wantName {
		t.Fatalf("template name = %q, want %q", got, wantName)
	}
	if got := len(template.Fields["fields"].Fields["map"].Map); got != wantFields {
		t.Fatalf("template field count = %d, want %d", got, wantFields)
	}
	if got := template.Fields["childRelationships"].List[0].Fields["junctionIdListNames"].List[0].Text; got != wantJunction {
		t.Fatalf("template junction name = %q, want %q", got, wantJunction)
	}
}

func TestExecSObjectDescribeCollectionsArePrivateWithoutOverrides(t *testing.T) {
	program, err := CompileAnonymous(`
Schema.DescribeSObjectResult deferred = Widget__c.SObjectType.getDescribe(SObjectDescribeOptions.DEFERRED);
System.assertEquals('Widget__c', deferred.getName());
System.assertEquals(SObjectDescribeOptions.DEFERRED, deferred.getSObjectDescribeOption());

Map<String, Schema.SObjectField> firstFields = Widget__c.SObjectType.getDescribe().fields.getMap();
Integer fieldCount = firstFields.size();
firstFields.clear();
System.assertEquals(fieldCount, Widget__c.SObjectType.getDescribe().fields.getMap().size());

List<Schema.RecordTypeInfo> firstRecordTypes = Widget__c.SObjectType.getDescribe().getRecordTypeInfos();
Integer recordTypeCount = firstRecordTypes.size();
firstRecordTypes.clear();
System.assertEquals(recordTypeCount, Widget__c.SObjectType.getDescribe().getRecordTypeInfos().size());

List<Schema.FieldSetMember> firstMembers = Widget__c.SObjectType.getDescribe().fieldSets.getMap().get('Summary').getFields();
Integer memberCount = firstMembers.size();
firstMembers.clear();
System.assertEquals(memberCount, Widget__c.SObjectType.getDescribe().fieldSets.getMap().get('Summary').getFields().size());

Map<String, Schema.FieldSet> firstFieldSets = Widget__c.SObjectType.getDescribe().fieldSets.getMap();
Integer fieldSetCount = firstFieldSets.size();
firstFieldSets.clear();
System.assertEquals(fieldSetCount, Widget__c.SObjectType.getDescribe().fieldSets.getMap().size());

List<Schema.ChildRelationship> firstChildren = Widget__c.SObjectType.getDescribe().getChildRelationships();
Integer childCount = firstChildren.size();
Schema.ChildRelationship firstChild = firstChildren[0];
firstChildren.clear();
System.assertEquals(childCount, Widget__c.SObjectType.getDescribe().getChildRelationships().size());

List<String> firstJunctionNames = firstChild.getJunctionIdListNames();
Integer junctionNameCount = firstJunctionNames.size();
firstJunctionNames.clear();
System.assertEquals(junctionNameCount, firstChild.getJunctionIdListNames().size());

List<Schema.SObjectType> firstJunctionTargets = firstChild.getJunctionReferenceTo();
Integer junctionTargetCount = firstJunctionTargets.size();
firstJunctionTargets.clear();
System.assertEquals(junctionTargetCount, firstChild.getJunctionReferenceTo().size());

List<Schema.SObjectType> firstReferences = Widget__c.Lookup__c.getDescribe().getReferenceTo();
String firstReferenceName = firstReferences[0].getDescribe().getName();
firstReferences[0] = Account.SObjectType;
System.assertEquals(firstReferenceName, Widget__c.Lookup__c.getDescribe().getReferenceTo()[0].getDescribe().getName());

List<Schema.PicklistEntry> firstPicklist = Widget__c.Status__c.getDescribe().getPicklistValues();
String firstPicklistValue = firstPicklist[0].getValue();
firstPicklist[0] = firstPicklist[1];
System.assertEquals(firstPicklistValue, Widget__c.Status__c.getDescribe().getPicklistValues()[0].getValue());

Map<String, Integer> firstControllerValues = Widget__c.Stage__c.getDescribe().getControllerValues();
Integer controllerValueCount = firstControllerValues.size();
firstControllerValues.clear();
System.assertEquals(controllerValueCount, Widget__c.Stage__c.getDescribe().getControllerValues().size());

Schema.FilteredLookupInfo firstLookupInfo = Widget__c.Lookup__c.getDescribe().getFilteredLookupInfo();
List<String> firstControllingFields = firstLookupInfo.getControllingFields();
String firstControllingField = firstControllingFields[0];
firstControllingFields[0] = 'Changed__c';
System.assertEquals(firstControllingField, Widget__c.Lookup__c.getDescribe().getFilteredLookupInfo().getControllingFields()[0]);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := describeOverlayOrg(4)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func BenchmarkSObjectDescribeDeepClone(b *testing.B) {
	template := describeOverlayBenchmarkFixture(b, 2000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		describe := cloneValue(template)
		if describe.Kind != ValueObject {
			b.Fatal("unexpected describe kind")
		}
	}
}

func BenchmarkSObjectDescribeOverlay(b *testing.B) {
	template := describeOverlayBenchmarkFixture(b, 2000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		describe := overlaySObjectDescribe(template, "Widget__c", "DEFERRED")
		if describe.Kind != ValueObject {
			b.Fatal("unexpected describe kind")
		}
	}
}

func describeOverlayBenchmarkFixture(tb testing.TB, fieldCount int) Value {
	tb.Helper()
	machine := New(nil)
	org := describeOverlayOrg(fieldCount)
	machine.SetOrg(&org)
	name, definition, ok := machine.describeObjectDefinition("pkg__Widget__c")
	if !ok {
		tb.Fatal("fixture object not found")
	}
	describe := machine.describeSObjectValue(name, definition)
	describe.Fields["childRelationships"] = List(machine.childRelationshipValue("pkg__Link__c", storage.Relationship{
		Field:               "Widget__c",
		ChildRelationship:   "Links",
		JunctionIDListNames: []string{"LinkIds"},
		JunctionReferenceTo: []string{"pkg__Link__c"},
	}))
	return describe
}

func describeOverlayFixture(t *testing.T, fieldCount int) Value {
	t.Helper()
	return describeOverlayBenchmarkFixture(t, fieldCount)
}

func describeOverlayOrg(fieldCount int) storage.OrgState {
	org := storage.NewOrgState()
	org.Namespace = "pkg"
	fields := make(map[string]storage.Field, fieldCount)
	for i := 0; i < fieldCount; i++ {
		name := fmt.Sprintf("pkg__Field_%04d__c", i)
		fields[name] = storage.Field{APIName: name, Label: name, Type: storage.FieldString}
	}
	fields["pkg__Status__c"] = storage.Field{
		APIName: "pkg__Status__c",
		Type:    storage.FieldPicklist,
		PicklistValues: []storage.PicklistValue{
			{Value: "Open", Label: "Open", Active: true},
			{Value: "Closed", Label: "Closed", Active: true},
		},
	}
	fields["pkg__Stage__c"] = storage.Field{
		APIName:            "pkg__Stage__c",
		Type:               storage.FieldPicklist,
		PicklistController: "pkg__Status__c",
		PicklistValues: []storage.PicklistValue{
			{Value: "New", Label: "New", Active: true},
		},
		PicklistValueSettings: []storage.PicklistSetting{{
			ValueName:              "New",
			ControllingFieldValues: []string{"Open"},
		}},
	}
	fields["pkg__Lookup__c"] = storage.Field{
		APIName:     "pkg__Lookup__c",
		Type:        storage.FieldReference,
		ReferenceTo: []string{"pkg__Widget__c", "pkg__Link__c"},
		FilteredLookupInfo: storage.FilteredLookupInfo{
			ControllingFields: []string{"pkg__Status__c"},
			Dependent:         true,
		},
	}
	org.Objects["pkg__Widget__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "pkg__Widget__c",
			Fields:  fields,
			RecordTypes: []storage.RecordTypeInfo{{
				ID:            storage.ID("012000000000001"),
				Name:          "Default",
				DeveloperName: "Default",
				Active:        true,
				Available:     true,
			}},
		},
		Records: map[storage.ID]storage.Record{},
	}
	org.Objects["Widget__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Widget__c",
			Fields: map[string]storage.Field{
				"Status__c": {
					APIName: "Status__c",
					Type:    storage.FieldPicklist,
					PicklistValues: []storage.PicklistValue{
						{Value: "Open", Label: "Open", Active: true},
						{Value: "Closed", Label: "Closed", Active: true},
					},
				},
				"Stage__c": {
					APIName:            "Stage__c",
					Type:               storage.FieldPicklist,
					PicklistController: "Status__c",
					PicklistValues: []storage.PicklistValue{
						{Value: "New", Label: "New", Active: true},
					},
					PicklistValueSettings: []storage.PicklistSetting{{
						ValueName:              "New",
						ControllingFieldValues: []string{"Open"},
					}},
				},
				"Lookup__c": {
					APIName:     "Lookup__c",
					Type:        storage.FieldReference,
					ReferenceTo: []string{"Widget__c", "pkg__Link__c"},
					FilteredLookupInfo: storage.FilteredLookupInfo{
						ControllingFields: []string{"Status__c"},
						Dependent:         true,
					},
				},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	org.Objects["pkg__Link__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "pkg__Link__c",
			Fields: map[string]storage.Field{
				"pkg__Widget__c": {
					APIName:     "pkg__Widget__c",
					Type:        storage.FieldReference,
					ReferenceTo: []string{"pkg__Widget__c"},
				},
			},
			Relations: []storage.Relationship{{
				Field:               "pkg__Widget__c",
				ParentObjects:       []string{"pkg__Widget__c"},
				ChildRelationship:   "pkg__Links__r",
				JunctionIDListNames: []string{"LinkIds"},
				JunctionReferenceTo: []string{"pkg__Link__c"},
			}},
		},
		Records: map[storage.ID]storage.Record{},
	}
	org.Metadata.FieldSets = []storage.FieldSetMetadata{{
		ObjectName: "Widget__c",
		Namespace:  "pkg",
		Name:       "Summary",
		Fields:     []storage.FieldSetMemberMetadata{{Field: "Field_0000__c"}},
	}}
	return org
}

func firstDescribeFieldSet(describe Value) Value {
	fieldSetMap := describe.Fields["fieldSets"].Fields["map"]
	for _, fieldSet := range fieldSetMap.Map {
		return fieldSet
	}
	return Null
}
