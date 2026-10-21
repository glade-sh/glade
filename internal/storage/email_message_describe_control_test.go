package storage

import (
	"encoding/json"
	"testing"
)

func TestEmailMessageOwnedDescribeNameField(t *testing.T) {
	var raw []byte
	for _, entry := range standardDescribeCatalogV2Index {
		if entry.Name == "EmailMessage" {
			var err error
			raw, err = decodeStandardDescribeCatalogV2MemberBytes(standardDescribeCatalogV2Pack, entry, standardDescribeCatalogV2Magic, len(standardDescribeCatalogV2Index))
			if err != nil {
				t.Fatal(err)
			}
			break
		}
	}
	var describe struct {
		Fields []struct {
			Name                              string
			NameField, Createable, Updateable bool
		}
	}
	if err := json.Unmarshal(raw, &describe); err != nil {
		t.Fatal(err)
	}
	foundName, foundSubject := false, false
	for _, field := range describe.Fields {
		if field.Name == "Name" {
			foundName = true
			if !field.NameField || field.Createable || field.Updateable {
				t.Fatalf("captured Name facets changed: %+v", field)
			}
		}
		if field.Name == "Subject" {
			foundSubject = true
			if field.NameField {
				t.Fatal("captured Subject became a name field")
			}
		}
	}
	if !foundName || !foundSubject {
		t.Fatal("captured EmailMessage fields missing")
	}
}
