package vm

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestDescribeChildPruningGenerationBoundaries(t *testing.T) {
	for _, boundary := range []string{"namespace", "object-count", "runtime-generation", "schema-generation", "install"} {
		t.Run(boundary, func(t *testing.T) {
			template := describePruningTemplate()
			machine := describePruningClone(template)
			_ = machine.describeChildRelationships("Root000__c")
			switch boundary {
			case "namespace":
				machine.Org.Namespace = "pkg"
			case "object-count":
				state := candidatePlanChild("Extra__c", "Root000__c", "Extras__r", 0)
				storage.EnsureStandardObjectFields(&state.Definition)
				machine.Org.Objects["Extra__c"] = state
			case "runtime-generation":
				machine.Org.InvalidateRuntimeGeneration()
			case "schema-generation":
				if _, ok := storage.EnsureMutableObjectDefinition(machine.Org, "Branch000__c"); !ok {
					t.Fatal("missing child")
				}
			case "install":
				org := machine.Org.Clone()
				machine.SetOrg(&org)
			}
			compareChildDescribeWithBaseline(t, machine, "Root000__c")
		})
	}
}

func TestDescribeChildPruningConcurrentClones(t *testing.T) {
	template := describePruningTemplate()
	control := describePruningClone(template)
	want := goldenChildDescribe(t, control, "Root000__c")
	var wg sync.WaitGroup
	errors := make(chan error, 4)
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			machine := describePruningClone(template)
			got, err := json.Marshal(machine.describeChildRelationships("Root000__c"))
			if err != nil {
				errors <- err
			} else if string(got) != string(want) {
				errors <- fmt.Errorf("concurrent clone differs from baseline")
			}
		}()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
}
