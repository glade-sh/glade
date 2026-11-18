package dml_test

import (
	"sync"
	"testing"

	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
)

// Queries on concurrent clones see their own setup-row updates, never a sibling's.
func TestClonedSetupRowsSOQLIsolation(t *testing.T) {
	template := storage.NewOrgState()
	storage.EnsureStandardObject(&template, "UserLicense")
	const id storage.ID = "100000000000001AAA"
	obj := template.Objects["UserLicense"]
	obj.PutRecord(id, storage.Record{ID: id, Object: "UserLicense", Fields: map[string]storage.Value{"Name": storage.StringValue("original")}})
	template.Objects["UserLicense"] = obj
	var wg sync.WaitGroup
	for _, name := range []string{"first", "second"} {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			clone := template.CloneRuntimeFrozenShared()
			mutable, _ := storage.EnsureMutableObjectRecords(&clone, "UserLicense")
			row := mutable.Records[id]
			row.Fields["Name"] = storage.StringValue(name)
			mutable.PutRecord(id, row)
			clone.Objects["UserLicense"] = *mutable
			result, err := soql.ParseAndExecute(clone, "SELECT Name FROM UserLicense")
			if err != nil {
				t.Error(err)
				return
			}
			if len(result.Records) != 1 || result.Records[0].Fields["Name"].String != name {
				t.Errorf("%s query returned %+v", name, result.Records)
			}
		}(name)
	}
	wg.Wait()
	result, err := soql.ParseAndExecute(template, "SELECT Name FROM UserLicense")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Records) != 1 || result.Records[0].Fields["Name"].String != "original" {
		t.Fatalf("template query returned %+v", result.Records)
	}
}
