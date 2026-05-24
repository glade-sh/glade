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

func TestApplyCustomMetadataRecordsSetsNamespacePrefixForLocalNamespacedRows(t *testing.T) {
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
	if got := record.Fields["QualifiedApiName"].String; got != "pkg__Default" {
		t.Fatalf("QualifiedApiName = %q, want pkg__Default", got)
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
