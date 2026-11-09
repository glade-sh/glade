package storage

import (
	"testing"

	"github.com/glade-sh/glade/internal/schema"
)

func TestApplyCustomMetadataRecordsAssignsDistinctPrefixesAcrossTypes(t *testing.T) {
	org := NewOrgState()
	org.Objects["FeatureA__mdt"] = ObjectState{
		Definition: ObjectDefinition{APIName: "FeatureA__mdt", Fields: map[string]Field{}},
		Records:    make(map[ID]Record),
	}
	org.Objects["FeatureB__mdt"] = ObjectState{
		Definition: ObjectDefinition{APIName: "FeatureB__mdt", Fields: map[string]Field{}},
		Records:    make(map[ID]Record),
	}

	err := ApplyCustomMetadataRecords(&org, []schema.CustomMetadataRecord{
		{FullName: "FeatureA.Default", ObjectName: "FeatureA__mdt", DeveloperName: "Default"},
		{FullName: "FeatureB.Default", ObjectName: "FeatureB__mdt", DeveloperName: "Default"},
	})
	if err != nil {
		t.Fatal(err)
	}

	a := onlyRecordID(t, org.Objects["FeatureA__mdt"].Records)
	b := onlyRecordID(t, org.Objects["FeatureB__mdt"].Records)
	if string(a[:3]) == string(b[:3]) {
		t.Fatalf("custom metadata prefixes collided: %s and %s", a, b)
	}
}

func TestApplyCustomMetadataRecordsAvoidsLowDeterministicTestIDs(t *testing.T) {
	org := NewOrgState()
	org.Objects["Feature__mdt"] = ObjectState{
		Definition: ObjectDefinition{APIName: "Feature__mdt", Fields: map[string]Field{}},
		Records:    make(map[ID]Record),
	}
	if err := ApplyCustomMetadataRecords(&org, []schema.CustomMetadataRecord{{
		FullName: "Feature.Default", ObjectName: "Feature__mdt", DeveloperName: "Default",
	}}); err != nil {
		t.Fatal(err)
	}
	id := onlyRecordID(t, org.Objects["Feature__mdt"].Records)
	if id == "m000000000000001" || id == "m00000000000001" {
		t.Fatalf("custom metadata record reused low deterministic test id: %s", id)
	}
}

func TestApplyCustomMetadataRecordsSetsNamespacePrefixAndLocalQualifiedNameForLocalNamespacedRows(t *testing.T) {
	org := NewOrgState()
	org.Namespace = "pkg"
	definition := ObjectDefinition{
		APIName:  "Feature__mdt",
		Metadata: map[string]string{"kind": "customMetadata"},
		Fields:   map[string]Field{},
	}
	EnsureStandardObjectFields(&definition)
	org.Objects["Feature__mdt"] = ObjectState{Definition: definition, Records: map[ID]Record{}}

	err := ApplyCustomMetadataRecords(&org, []schema.CustomMetadataRecord{
		{FullName: "Feature.Default", ObjectName: "Feature__mdt", DeveloperName: "Default"},
	})
	if err != nil {
		t.Fatal(err)
	}

	record := onlyRecord(t, org.Objects["Feature__mdt"].Records)
	if got := record.Fields["NamespacePrefix"].String; got != "pkg" {
		t.Fatalf("NamespacePrefix = %q, want pkg", got)
	}
	if got := record.Fields["QualifiedApiName"].String; got != "Default" {
		t.Fatalf("QualifiedApiName = %q, want Default", got)
	}
}

func TestApplyCustomMetadataRecordsUsesObjectNamespaceForDependencyQualifiedName(t *testing.T) {
	org := NewOrgState()
	org.Namespace = "consumer"
	definition := ObjectDefinition{
		APIName:  "pkg__Feature__mdt",
		Metadata: map[string]string{"kind": "customMetadata"},
		Fields:   map[string]Field{},
	}
	EnsureStandardObjectFields(&definition)
	org.Objects["pkg__Feature__mdt"] = ObjectState{Definition: definition, Records: map[ID]Record{}}

	err := ApplyCustomMetadataRecords(&org, []schema.CustomMetadataRecord{
		{FullName: "pkg__Feature.Default", ObjectName: "pkg__Feature__mdt", DeveloperName: "Default"},
	})
	if err != nil {
		t.Fatal(err)
	}

	record := onlyRecord(t, org.Objects["pkg__Feature__mdt"].Records)
	if got := record.Fields["NamespacePrefix"].String; got != "pkg" {
		t.Fatalf("NamespacePrefix = %q, want pkg", got)
	}
	if got := record.Fields["QualifiedApiName"].String; got != "pkg__Default" {
		t.Fatalf("QualifiedApiName = %q, want pkg__Default", got)
	}
}

func TestApplyCustomMetadataRecordsDefaultsOmittedCheckboxFieldsToFalse(t *testing.T) {
	org := NewOrgState()
	definition := ObjectDefinition{
		APIName:  "Feature__mdt",
		Metadata: map[string]string{"kind": "customMetadata"},
		Fields: map[string]Field{
			"Is_Deleted__c": {APIName: "Is_Deleted__c", Type: FieldBoolean},
		},
	}
	EnsureStandardObjectFields(&definition)
	org.Objects["Feature__mdt"] = ObjectState{Definition: definition, Records: map[ID]Record{}}

	err := ApplyCustomMetadataRecords(&org, []schema.CustomMetadataRecord{
		{FullName: "Feature.Default", ObjectName: "Feature__mdt", DeveloperName: "Default"},
	})
	if err != nil {
		t.Fatal(err)
	}

	record := onlyRecord(t, org.Objects["Feature__mdt"].Records)
	if got := record.Fields["Is_Deleted__c"]; got.Kind != ValueBoolean || got.Boolean {
		t.Fatalf("Is_Deleted__c = %#v, want false checkbox value", got)
	}
}

func TestApplyCustomMetadataRecordsRebuildsRelationshipIndexesAfterResolvingRefs(t *testing.T) {
	org := NewOrgState()
	parentDefinition := ObjectDefinition{
		APIName:  "StateConfiguration__mdt",
		Metadata: map[string]string{"kind": "customMetadata"},
		Fields:   map[string]Field{},
	}
	childDefinition := ObjectDefinition{
		APIName:  "StateTransition__mdt",
		Metadata: map[string]string{"kind": "customMetadata"},
		Fields: map[string]Field{
			"StateConfiguration__c": {
				APIName:          "StateConfiguration__c",
				Type:             FieldReference,
				ReferenceTo:      []string{"StateConfiguration__mdt"},
				RelationshipName: "StateConfiguration__r",
			},
		},
		Indexes: []IndexDefinition{{
			Name:   "StateTransition__mdt.StateConfiguration__c",
			Object: "StateTransition__mdt",
			Fields: []string{"StateConfiguration__c"},
		}},
	}
	EnsureStandardObjectFields(&parentDefinition)
	EnsureStandardObjectFields(&childDefinition)
	org.Objects["StateConfiguration__mdt"] = ObjectState{Definition: parentDefinition, Records: map[ID]Record{}}
	org.Objects["StateTransition__mdt"] = ObjectState{Definition: childDefinition, Records: map[ID]Record{}}

	err := ApplyCustomMetadataRecords(&org, []schema.CustomMetadataRecord{
		{FullName: "StateConfiguration.OrderGraph", ObjectName: "StateConfiguration__mdt", DeveloperName: "OrderGraph"},
		{FullName: "StateTransition.order_submit_as_proforma", ObjectName: "StateTransition__mdt", DeveloperName: "order_submit_as_proforma", Values: []schema.CustomMetadataValue{
			{Field: "StateConfiguration__c", Value: "OrderGraph"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	parentID := onlyRecordID(t, org.Objects["StateConfiguration__mdt"].Records)
	ids, ok := LookupIndex(org.Objects["StateTransition__mdt"], "StateConfiguration__c", IDValue(parentID))
	if !ok || len(ids) != 1 {
		t.Fatalf("relationship index = %#v ok=%v", ids, ok)
	}
}

func TestApplyCustomMetadataRecordsResolvesDependencyRelationshipByTargetType(t *testing.T) {
	org := NewOrgState()
	org.Namespace = "local"
	callbackDefinition := ObjectDefinition{
		APIName:  "pkg__Callback__mdt",
		Metadata: map[string]string{"kind": "customMetadata"},
		Fields:   map[string]Field{},
	}
	transitionDefinition := ObjectDefinition{
		APIName:  "pkg__Transition__mdt",
		Metadata: map[string]string{"kind": "customMetadata"},
		Fields:   map[string]Field{},
	}
	bindingDefinition := ObjectDefinition{
		APIName:  "pkg__TransitionCallback__mdt",
		Metadata: map[string]string{"kind": "customMetadata"},
		Fields: map[string]Field{
			"pkg__Callback__c": {
				APIName:          "pkg__Callback__c",
				Type:             FieldReference,
				ReferenceTo:      []string{"pkg__Callback__mdt"},
				RelationshipName: "pkg__Callback__r",
			},
			"pkg__Transition__c": {
				APIName:          "pkg__Transition__c",
				Type:             FieldReference,
				ReferenceTo:      []string{"pkg__Transition__mdt"},
				RelationshipName: "pkg__Transition__r",
			},
		},
	}
	EnsureStandardObjectFields(&callbackDefinition)
	EnsureStandardObjectFields(&transitionDefinition)
	EnsureStandardObjectFields(&bindingDefinition)
	org.Objects["pkg__Callback__mdt"] = ObjectState{Definition: callbackDefinition, Records: map[ID]Record{}}
	org.Objects["pkg__Transition__mdt"] = ObjectState{Definition: transitionDefinition, Records: map[ID]Record{}}
	org.Objects["pkg__TransitionCallback__mdt"] = ObjectState{Definition: bindingDefinition, Records: map[ID]Record{}}

	err := ApplyCustomMetadataRecords(&org, []schema.CustomMetadataRecord{
		{FullName: "pkg__Callback.Generate", ObjectName: "pkg__Callback__mdt", DeveloperName: "Generate"},
		{FullName: "pkg__Transition.Submit", ObjectName: "pkg__Transition__mdt", DeveloperName: "Submit"},
		{FullName: "pkg__TransitionCallback.GenerateOnSubmit", ObjectName: "pkg__TransitionCallback__mdt", DeveloperName: "GenerateOnSubmit", Values: []schema.CustomMetadataValue{
			{Field: "Callback__c", Value: "Callback.Generate"},
			{Field: "Transition__c", Value: "Transition.Submit"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	callbackID := onlyRecordID(t, org.Objects["pkg__Callback__mdt"].Records)
	transitionID := onlyRecordID(t, org.Objects["pkg__Transition__mdt"].Records)
	binding := onlyRecord(t, org.Objects["pkg__TransitionCallback__mdt"].Records)
	if got := binding.Fields["pkg__Callback__c"]; got.Kind != ValueID || got.ID != callbackID {
		t.Fatalf("Callback__c = %#v, want %s", got, callbackID)
	}
	if got := binding.Fields["pkg__Transition__c"]; got.Kind != ValueID || got.ID != transitionID {
		t.Fatalf("Transition__c = %#v, want %s", got, transitionID)
	}
}

func onlyRecordID(t *testing.T, records map[ID]Record) ID {
	t.Helper()
	if len(records) != 1 {
		t.Fatalf("records = %d, want 1", len(records))
	}
	for id := range records {
		return id
	}
	return ""
}

// Explicit record ownership is independent of the packaged type namespace.
// Missing provenance retains the legacy type-based fallback.
func TestCustomMetadataFieldsUsesCapturedRecordOwnerProvenance(t *testing.T) {
	empty, provider, other := "", "provider", "other"
	for _, tc := range []struct {
		name          string
		owner         *string
		wantNamespace Value
		wantQualified string
	}{
		{"subscriber", &empty, NullValue(), "Seed"},
		{"provider", &provider, StringValue("provider"), "provider__Seed"},
		{"legacy", nil, StringValue("provider"), "provider__Seed"},
		{"other owner", &other, StringValue("other"), "other__Seed"},
	} {
		for _, namespace := range []string{"", "other"} {
			t.Run(tc.name+"/org="+namespace, func(t *testing.T) {
				org := NewOrgState()
				org.Namespace = namespace
				fields, _, err := customMetadataFields(org, ObjectDefinition{APIName: "provider__Flag__mdt"}, schema.CustomMetadataRecord{DeveloperName: "Seed", OwnerNamespace: tc.owner})
				if err != nil {
					t.Fatal(err)
				}
				if got := fields["NamespacePrefix"]; got.Kind != tc.wantNamespace.Kind || got.String != tc.wantNamespace.String {
					t.Fatalf("owner = %#v, want %#v", got, tc.wantNamespace)
				}
				if got := fields["QualifiedApiName"]; got.Kind != ValueString || got.String != tc.wantQualified {
					t.Fatalf("qualified name = %#v, want %q", got, tc.wantQualified)
				}
			})
		}
	}
}

// Native rows (scratch org, API 62.0, 67.0, 68.0): a deployed custom metadata
// record queried and read back Language = en_US, and describe reported Language
// as a PICKLIST of length 40 on the type.
func TestApplyCustomMetadataRecordsSetsLanguageToOrgDefault(t *testing.T) {
	org := NewOrgState()
	definition := ObjectDefinition{
		APIName:  "Feature__mdt",
		Metadata: map[string]string{"kind": "customMetadata"},
		Fields:   map[string]Field{},
	}
	EnsureStandardObjectFields(&definition)
	if got := definition.Fields["Language"].Type; got != FieldPicklist {
		t.Fatalf("Language field type = %q, want picklist", got)
	}
	org.Objects["Feature__mdt"] = ObjectState{Definition: definition, Records: map[ID]Record{}}
	err := ApplyCustomMetadataRecords(&org, []schema.CustomMetadataRecord{
		{FullName: "Feature.Default", ObjectName: "Feature__mdt", DeveloperName: "Default"},
	})
	if err != nil {
		t.Fatal(err)
	}
	record := onlyRecord(t, org.Objects["Feature__mdt"].Records)
	if got := record.Fields["Language"]; got.Kind != ValueString || got.String != "en_US" {
		t.Fatalf("Language = %#v, want en_US", got)
	}
}

func TestApplyCustomMetadataRecordsCanonicalNameRouteTwins(t *testing.T) {
	for _, namespace := range []string{"", "sprout"} {
		for _, qualified := range []bool{false, true} {
			name := "DefaultProbe__mdt"
			if namespace != "" {
				name = namespace + "__" + name
			}
			route := "local-record-name"
			input := "DefaultProbe__mdt"
			if qualified {
				input = name
				route = "qualified-record-name"
			}
			t.Run(namespace+"/"+route, func(t *testing.T) {
				org := NewOrgState()
				org.Namespace = namespace
				org.Objects[name] = ObjectState{Definition: ObjectDefinition{APIName: name, KeyPrefix: "m09", Fields: map[string]Field{}}}
				if err := ApplyCustomMetadataRecords(&org, []schema.CustomMetadataRecord{{ObjectName: input, DeveloperName: "Product"}}); err != nil {
					t.Fatal(err)
				}
				if len(org.Objects) != 1 {
					t.Fatalf("prefix preparation introduced schema aliases: %+v", org.Objects)
				}
				state := org.Objects[name]
				if state.Definition.KeyPrefix != "m09" || len(state.Records) != 1 {
					t.Fatalf("canonical type changed: %+v", state)
				}
				for id, row := range state.Records {
					if string(id[:3]) != "m09" || row.Object != name {
						t.Fatalf("record identity changed: %+v", row)
					}
				}
			})
		}
	}
}

func TestApplyCustomMetadataRecordsQualifiesLocalEntityAndFieldDefinitionNames(t *testing.T) {
	org := NewOrgState()
	org.Namespace = "pkg"
	local := ObjectDefinition{APIName: "pkg__Delivery__c", Fields: map[string]Field{
		"pkg__Status__c": {APIName: "pkg__Status__c", Type: FieldString},
	}}
	EnsureStandardObjectFields(&local)
	org.Objects["pkg__Delivery__c"] = ObjectState{Definition: local, Records: map[ID]Record{}}
	bucket := ObjectDefinition{APIName: "pkg__Grouped__mdt", Metadata: map[string]string{"kind": "customMetadata"}, Fields: map[string]Field{
		"pkg__Object__c": {APIName: "pkg__Object__c", Type: FieldReference, ReferenceTo: []string{"EntityDefinition"}},
		"pkg__Field__c": {APIName: "pkg__Field__c", Type: FieldReference, ReferenceTo: []string{"FieldDefinition"},
			MetadataRelationshipControllingField: "Grouped__mdt.Object__c"},
	}}
	EnsureStandardObjectFields(&bucket)
	org.Objects["pkg__Grouped__mdt"] = ObjectState{Definition: bucket, Records: map[ID]Record{}}

	err := ApplyCustomMetadataRecords(&org, []schema.CustomMetadataRecord{
		{FullName: "Grouped.Local", ObjectName: "pkg__Grouped__mdt", DeveloperName: "Local", Values: []schema.CustomMetadataValue{
			{Field: "Object__c", Value: "Delivery__c"}, {Field: "Field__c", Value: "Status__c"}}},
		{FullName: "Grouped.Other", ObjectName: "pkg__Grouped__mdt", DeveloperName: "Other", Values: []schema.CustomMetadataValue{
			{Field: "Object__c", Value: "Elsewhere__c"}, {Field: "Field__c", Value: "Status__c"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range org.Objects["pkg__Grouped__mdt"].Records {
		object, field := record.Fields["pkg__Object__c"].String, record.Fields["pkg__Field__c"].String
		switch record.Fields["DeveloperName"].String {
		case "Local":
			if object != "pkg__Delivery__c" || field != "pkg__Status__c" {
				t.Fatalf("local = %q, %q", object, field)
			}
		case "Other":
			if object != "Elsewhere__c" || field != "Status__c" {
				t.Fatalf("other = %q, %q", object, field)
			}
		}
	}
}

func TestApplyCustomMetadataRecordsResolvesFifteenAndEighteenLetterDeveloperNamesInAnyOrder(t *testing.T) {
	names := []string{"AlphaBetaGamma1", "AlphaBetaGammaDel1", "AlphaBetaGammaDeltaEps"}
	for _, name := range names {
		org := NewOrgState()
		parent := ObjectDefinition{APIName: "Parent__mdt", Metadata: map[string]string{"kind": "customMetadata"}, Fields: map[string]Field{}}
		child := ObjectDefinition{APIName: "Child__mdt", Metadata: map[string]string{"kind": "customMetadata"}, Fields: map[string]Field{
			"Parent__c": {APIName: "Parent__c", Type: FieldReference, ReferenceTo: []string{"Parent__mdt"}, RelationshipName: "Parent__r"},
		}}
		EnsureStandardObjectFields(&parent)
		EnsureStandardObjectFields(&child)
		org.Objects["Parent__mdt"] = ObjectState{Definition: parent, Records: map[ID]Record{}}
		org.Objects["Child__mdt"] = ObjectState{Definition: child, Records: map[ID]Record{}}
		childRecord := schema.CustomMetadataRecord{FullName: "Child.One", ObjectName: "Child__mdt", DeveloperName: "One",
			Values: []schema.CustomMetadataValue{{Field: "Parent__c", Value: name}}}
		// The child alone cannot resolve: the parent is not loaded yet.
		if err := ApplyCustomMetadataRecords(&org, []schema.CustomMetadataRecord{childRecord}); err == nil {
			t.Fatalf("%s (%d letters): child applied before its parent existed", name, len(name))
		}
		org = NewOrgState()
		org.Objects["Parent__mdt"] = ObjectState{Definition: parent, Records: map[ID]Record{}}
		org.Objects["Child__mdt"] = ObjectState{Definition: child, Records: map[ID]Record{}}
		err := ApplyCustomMetadataRecords(&org, []schema.CustomMetadataRecord{
			childRecord,
			{FullName: "Parent." + name, ObjectName: "Parent__mdt", DeveloperName: name},
		})
		if err != nil {
			t.Fatal(err)
		}
		parentID := onlyRecordID(t, org.Objects["Parent__mdt"].Records)
		childID := onlyRecordID(t, org.Objects["Child__mdt"].Records)
		if got := org.Objects["Child__mdt"].Records[childID].Fields["Parent__c"]; got.ID != parentID {
			t.Fatalf("%s: Parent__c = %#v, want %s", name, got, parentID)
		}
	}
}

func TestResolveCustomMetadataRecordIDRawIDBoundary(t *testing.T) {
	build := func(keyPrefix string) OrgState {
		org := NewOrgState()
		def := ObjectDefinition{APIName: "Parent__mdt", KeyPrefix: keyPrefix, Metadata: map[string]string{"kind": "customMetadata"}, Fields: map[string]Field{}}
		EnsureStandardObjectFields(&def)
		org.Objects["Parent__mdt"] = ObjectState{Definition: def, Records: map[ID]Record{}}
		return org
	}
	cases := []struct {
		name      string
		keyPrefix string
		raw       string
		want      bool
	}{
		{"matching prefix, 15 characters", "m09", "m09000000000001", true},
		{"matching prefix, 18 characters", "m09", "m09000000000001AAA", true},
		{"non-matching prefix", "m09", "m0A000000000001", false},
		{"empty KeyPrefix", "", "m09000000000001", false},
		{"15-letter name", "m09", "AlphaBetaGamma1", false},
		{"18-letter name", "m09", "AlphaBetaGammaDel1", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			org := build(tc.keyPrefix)
			got, ok := resolveCustomMetadataRecordID(org, tc.raw, []string{"Parent__mdt"})
			if ok != tc.want {
				t.Fatalf("resolveCustomMetadataRecordID(%q) ok = %v, want %v", tc.raw, ok, tc.want)
			}
			if ok && string(got) != tc.raw {
				t.Fatalf("id = %q, want %q", got, tc.raw)
			}
		})
	}
}
