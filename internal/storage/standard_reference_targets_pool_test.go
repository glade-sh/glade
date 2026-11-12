package storage

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
)

// The merge shares a pooled scratch set across goroutines. Concurrent calls
// must see exactly the single-threaded result.
func TestMergeStandardReferenceTargetsConcurrentUse(t *testing.T) {
	existing := make([]string, 0, 300)
	for i := 0; i < 300; i++ {
		name := fmt.Sprintf("Parent%dC", i%250)
		if i%3 == 0 {
			name = fmt.Sprintf("parent%dc", i%250)
		}
		existing = append(existing, name)
	}
	standard := []string{"Account", "Contact__c"}
	want := mergeStandardReferenceTargets(existing, standard)
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				got := mergeStandardReferenceTargets(existing, standard)
				if !reflect.DeepEqual(got, want) {
					t.Errorf("concurrent merge differs from serial result")
					return
				}
			}
		}()
	}
	wg.Wait()
}
