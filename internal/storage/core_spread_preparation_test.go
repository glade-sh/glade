package storage

import (
	"encoding/json"
	"reflect"
	"sync"
	"testing"
)

// legacyCoreSpreadPreparation reproduces the original setup for comparison.
func legacyCoreSpreadPreparation(o *OrgState) {
	markExistingExplicitBeforeSpread(o)
	o.ensureFeatureProfile()
	if o.Objects == nil {
		o.Objects = make(map[string]ObjectState)
	}
	o.StandardSpread = standardSpreadPlain
	o.ensureSpreadSuppression()
	if _, ok := o.Objects["RecordType"]; !ok {
		ensureRecordTypeObject(o)
	}
	for _, name := range standardSpreadCoreObjects {
		if standardSpreadIncludes(name) {
			if _, exists := o.Objects[name]; !exists {
				ensureStandardObjectWithProfile(o, name, nil)
			}
		}
	}
}

func TestCoreSpreadPreparationParity(t *testing.T) {
	for _, features := range [][]string{nil, {"MultiCurrency", "PersonAccounts"}, {"Communities", "NetworksEnabled", "Quotes"}} {
		for _, existing := range []bool{false, true} {
			orgs := [2]OrgState{NewOrgState(), NewOrgState()}
			for i := range orgs {
				if existing {
					EnsureStandardObject(&orgs[i], "Account")
					object := orgs[i].Objects["Account"]
					object.Definition.Fields["Example__c"] = Field{APIName: "Example__c", Type: FieldString}
					orgs[i].Objects["Account"] = object
				}
			}
			legacyCoreSpreadPreparation(&orgs[0])
			orgs[1].EnableStandardSpreadSource()
			for _, stage := range []string{"initial", "shaped", "complete"} {
				if stage == "shaped" {
					for i := range orgs {
						ApplyProjectOrgShape(&orgs[i], features)
					}
				}
				if stage == "complete" {
					for i := range orgs {
						MaterializeStandardObjectSpread(&orgs[i])
					}
				}
				left, err := json.Marshal(orgs[0])
				if err != nil {
					t.Fatal(err)
				}
				right, err := json.Marshal(orgs[1])
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(left, right) {
					t.Fatalf("features %v existing %v stage %s differs", features, existing, stage)
				}
			}
		}
	}
}

func TestCoreSpreadPreparationConcurrent(t *testing.T) {
	baseline := NewOrgState()
	legacyCoreSpreadPreparation(&baseline)
	want, err := json.Marshal(baseline)
	if err != nil {
		t.Fatal(err)
	}
	resetSpreadMemos(t)
	var wg sync.WaitGroup
	results := make([][]byte, 4)
	errs := make([]error, len(results))
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			org := NewOrgState()
			org.EnableStandardSpreadSource()
			results[i], errs[i] = json.Marshal(org)
		}(i)
	}
	wg.Wait()
	for i := range results {
		if errs[i] != nil || !reflect.DeepEqual(results[i], want) {
			t.Fatalf("concurrent org %d differs: %v", i, errs[i])
		}
	}
}

func BenchmarkCoreSpreadPreparation(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		org := NewOrgState()
		org.EnableStandardSpreadSource()
		if len(org.Objects) == 0 {
			b.Fatal("empty org")
		}
	}
}
