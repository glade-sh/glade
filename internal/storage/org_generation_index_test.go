package storage

import (
	"fmt"
	"sync"
	"testing"
)

func TestRuntimeGenerationInvalidationIsCloneLocal(t *testing.T) {
	org := NewOrgState()
	org.RuntimeSchemaStamp = "frozen-generation"
	template := NewRuntimeTemplate(org)
	template.RuntimeSchemaStamp = org.RuntimeSchemaStamp
	wantID := template.Org.RuntimeGenerationID()
	wantValue := new(int)
	template.Org.StoreRuntimeGenerationCache("index", wantValue)
	clone := template.CloneRuntimeOrg()
	clone.InvalidateRuntimeGeneration()
	if got := clone.RuntimeGenerationID(); got == wantID || got == 0 {
		t.Errorf("invalidated clone generation = %d, want a fresh nonzero ID", got)
	}
	if _, ok := clone.LoadRuntimeGenerationCache("index"); ok {
		t.Error("invalidated clone retained the old index")
	}
	clone.StoreRuntimeGenerationCache("index", new(int))
	sibling := template.CloneRuntimeOrg()
	for name, state := range map[string]*OrgState{"template": &template.Org, "sibling": &sibling} {
		if got := state.RuntimeGenerationID(); got != wantID {
			t.Errorf("%s generation = %d, want unchanged %d", name, got, wantID)
		}
		if got, ok := state.LoadRuntimeGenerationCache("index"); !ok || got != wantValue {
			t.Errorf("%s lost its shared index", name)
		}
	}
}

func TestRuntimeGenerationParallelCloneInvalidation(t *testing.T) {
	org := NewOrgState()
	org.RuntimeSchemaStamp = "frozen-generation"
	template := NewRuntimeTemplate(org)
	template.RuntimeSchemaStamp = org.RuntimeSchemaStamp
	wantID := template.Org.RuntimeGenerationID()
	wantValue := new(int)
	template.Org.StoreRuntimeGenerationCache("index", wantValue)
	var workers sync.WaitGroup
	start := make(chan struct{})
	for worker := 0; worker < 8; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			clone := template.CloneRuntimeOrg()
			<-start
			for i := 0; i < 256; i++ {
				if worker%2 == 0 {
					clone.InvalidateRuntimeGeneration()
					clone.RuntimeGenerationID()
				} else {
					if got := clone.RuntimeGenerationID(); got != wantID {
						t.Errorf("sibling generation = %d, want unchanged %d", got, wantID)
						return
					}
					if got, ok := clone.LoadRuntimeGenerationCache("index"); !ok || got != wantValue {
						t.Error("parallel invalidation discarded a sibling's index")
						return
					}
				}
			}
		}()
	}
	close(start)
	workers.Wait()
}

func TestRuntimeTemplateClonesReuseKeyPrefixValidation(t *testing.T) {
	org := NewOrgState()
	for i := 0; i < 256; i++ {
		name := fmt.Sprintf("Ready_%03d__c", i)
		org.Objects[name] = ObjectState{Definition: ObjectDefinition{APIName: name, KeyPrefix: customPrefix(i + 1000)}}
	}
	EnsureUniqueKeyPrefixes(&org)
	template := NewRuntimeTemplate(org)
	template.RuntimeSchemaStamp = "frozen-generation"
	template.Org.RuntimeSchemaStamp = template.RuntimeSchemaStamp
	before := keyPrefixSnapshotValidationCalls.Load()
	for i := 0; i < 3; i++ {
		clone := template.CloneRuntimeOrg()
		EnsureUniqueKeyPrefixes(&clone)
	}
	if got := keyPrefixSnapshotValidationCalls.Load() - before; got != 0 {
		t.Fatalf("frozen org clone prefix snapshot checks = %d, want 0", got)
	}
}
