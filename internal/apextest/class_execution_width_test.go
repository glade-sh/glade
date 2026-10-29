package apextest

import (
	"fmt"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

// Bound simultaneous owners across classes, not just each class's method tail.
// Yield at test_start so the old oversubscription is deterministic on one CPU.
func TestClassWorkersRespectExecutionWidth(t *testing.T) {
	previous := runtime.GOMAXPROCS(1)
	t.Cleanup(func() { runtime.GOMAXPROCS(previous) })
	for _, setup := range []bool{false, true} {
		for _, api := range []string{"62.0", "67.0"} {
			t.Run(fmt.Sprintf("setup=%v/api=%s", setup, api), func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
				for i := range 4 {
					name := fmt.Sprintf("Width%dTest", i)
					seed := ""
					if setup {
						seed = "@TestSetup static void seed() { insert new Contact(LastName='seed'); }"
					}
					writeFile(t, filepath.Join(root, "force-app/main/default/classes", name+".cls"), "@IsTest private class "+name+" {"+seed+`
@IsTest static void first() { System.assertEquals(0,[SELECT COUNT() FROM Account]); insert new Account(Name='first'); }
@IsTest static void second() { System.assertEquals(0,[SELECT COUNT() FROM Account]); insert new Account(Name='second'); }
}`)
				}
				var mu sync.Mutex
				active, peak := 0, 0
				opts := Options{Parallelism: 12, ParallelMethods: true, NoDiskCache: true, Progress: func(event TestProgress) {
					mu.Lock()
					switch event.Event {
					case "test_start":
						active++
						peak = max(peak, active)
					case "test_done":
						active--
					}
					mu.Unlock()
					if event.Event == "test_start" {
						runtime.Gosched()
					}
				}}
				index := loadTestIndex(t, root)
				done := make(chan struct{})
				go func() {
					defer close(done)
					run := Run(index, opts)
					if summary := run.Summary(); summary.Total != 8 || summary.Passed != 8 {
						t.Errorf("summary=%+v suites=%+v", summary, run.Suites)
					}
				}()
				<-done
				if active != 0 || peak != 1 {
					t.Fatalf("single CPU active=%d peak=%d; want 0/1", active, peak)
				}
			})
		}
	}
}
