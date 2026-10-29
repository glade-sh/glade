package vm

import (
	"fmt"
	"sync"
	"testing"
)

// Clones of one primed runtime share the global describe keys and order
// (copy-on-write). A typed binding to a differently spelled map type rewrites
// the entries; that rewrite must never reach storage another clone reads. Run
// under -race: before the fix the binding cleared and refilled the shared key
// map in place while a parallel test method of another clone iterated it.
func TestGlobalDescribeTypedRebindDoesNotWriteSharedKeys(t *testing.T) {
	for _, namespaced := range []bool{false, true} {
		t.Run(fmt.Sprintf("namespaced=%v", namespaced), func(t *testing.T) {
			base, template := describeSharePrimedBaseFor(t, namespaced)
			writer := describeShareClone(base, template)
			reader := describeShareClone(base, template)
			writerView, err := writer.coerceAssignable("Map<String,Schema.SObjectType>", writer.globalDescribeMapValue())
			if err != nil {
				t.Fatal(err)
			}
			readerView, err := reader.coerceAssignable("Map<String,Schema.SObjectType>", reader.globalDescribeMapValue())
			if err != nil {
				t.Fatal(err)
			}
			if !sameMapBacking(writerView.MapKeys, readerView.MapKeys) {
				t.Skip("clones no longer share global describe keys")
			}
			wantKeys := len(readerView.MapKeys)
			start := make(chan struct{})
			var wg sync.WaitGroup
			var rebindErr error
			wg.Add(2)
			go func() {
				defer wg.Done()
				<-start
				for range 200 {
					rebound, err := writer.coerceAssignable("Map<String,SObjectType>", writerView)
					if err != nil {
						rebindErr = err
						return
					}
					if len(rebound.MapKeys) != wantKeys {
						rebindErr = fmt.Errorf("rebound keys = %d, want %d", len(rebound.MapKeys), wantKeys)
						return
					}
				}
			}()
			counted := 0
			go func() {
				defer wg.Done()
				<-start
				for range 200 {
					for range readerView.MapKeys {
						counted++
					}
					for _, key := range readerView.MapOrder {
						if _, ok := readerView.Map[key]; !ok {
							counted = -1 << 30
						}
					}
				}
			}()
			close(start)
			wg.Wait()
			if rebindErr != nil {
				t.Fatal(rebindErr)
			}
			if counted != 200*wantKeys {
				t.Fatalf("reader saw %d keys, want %d: shared global describe storage changed under it", counted, 200*wantKeys)
			}
		})
	}
}
