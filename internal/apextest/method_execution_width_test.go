package apextest

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Extra method workers each own an org and journal. A single execution CPU
// must not create a tail of idle method workers solely because class slots
// became free. Clone counts are local guards; isolation remains unchanged.
func TestMethodWorkersRespectExecutionWidth(t *testing.T) {
	previous := runtime.GOMAXPROCS(1)
	t.Cleanup(func() { runtime.GOMAXPROCS(previous) })
	for _, api := range []string{"62.0", "67.0"} {
		for _, history := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/history=%v", api, history), func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
				classes := filepath.Join(root, "force-app/main/default/classes")
				var methods strings.Builder
				for i := 0; i < 12; i++ {
					fmt.Fprintf(&methods, `@IsTest static void case%d() {
System.assertEquals(0,[SELECT COUNT() FROM Account]);
insert new Account(Name='isolated');
System.assertEquals(1,[SELECT COUNT() FROM Account]);
}`, i)
				}
				writeFile(t, filepath.Join(classes, "ExecutionWidthTest.cls"), "@IsTest private class ExecutionWidthTest {"+methods.String()+"}")
				writeFile(t, filepath.Join(classes, "ExecutionSeedTest.cls"), `@IsTest private class ExecutionSeedTest {
@TestSetup static void seed() {insert new Contact(LastName='fixture');}
@IsTest static void observe(){System.assertEquals(1,[SELECT COUNT() FROM Contact]);}
}`)
				for _, name := range []string{"ExecutionWidthTest", "ExecutionSeedTest"} {
					writeFile(t, filepath.Join(classes, name+".cls-meta.xml"), "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
				}
				opts := Options{Parallelism: 12, ParallelMethods: true, NoDiskCache: true, PerfCounters: true}
				if history {
					opts.ClassDurationMS = map[string]int64{"ExecutionWidthTest": 100_000, "ExecutionSeedTest": 1_000}
				}
				index := loadTestIndex(t, root)
				// Exercise the asynchronous request boundary explicitly so the race
				// inventory includes the production-worker route under this guard.
				finished := make(chan string, 1)
				go func() {
					run := Run(index, opts)
					if summary := run.Summary(); summary.Total != 13 || summary.Passed != 13 {
						finished <- fmt.Sprintf("summary=%+v suites=%+v", summary, run.Suites)
						return
					}
					if clones := SnapshotPerfCounters().CloneRuntimeOrgCalls; clones > 3 {
						finished <- fmt.Sprintf("single-CPU org clones=%d, want at most 3", clones)
						return
					}
					finished <- ""
				}()
				if failure := <-finished; failure != "" {
					t.Fatal(failure)
				}
			})
		}
	}
}
