package apextest

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/testreport"
)

// Use the same marked global COW fixture as the anonymous VM route, including
// actual static/instance/parameter writeback and saved live key sets. Two CPUs
// and two requested workers exercise simultaneous methods; each method also
// verifies token state was not retained from a preceding pooled worker.
func TestDescribeHolderProjectConcurrentRoutes(t *testing.T) {
	old := runtime.GOMAXPROCS(2)
	defer runtime.GOMAXPROCS(old)
	raw, err := os.ReadFile("../vm/testdata/describe_ownership/holder.apex")
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, namespace := range []string{"", "zzns"} {
			t.Run(api+"/"+namespace, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q,"namespace":%q}`, api, namespace))
				var methods strings.Builder
				methods.WriteString(`public static Map<String,Schema.SObjectType> stored; public Map<String,Schema.SObjectType> member;`)
				id := 0
				for _, write := range []struct{ mutation, size string }{
					{"put('private',Account.SObjectType)", "beforeSize+1"},
					{"put('account',User.SObjectType)", "beforeSize"},
					{"putAll(new Map<String,Schema.SObjectType>{'private'=>Account.SObjectType})", "beforeSize+1"},
					{"remove('account')", "beforeSize-1"}, {"clear()", "0"},
					{"keySet().remove('account')", "beforeSize"}, {"keySet().clear()", "beforeSize"},
					{"keySet().removeAll(new Set<String>{'account'})", "beforeSize"},
					{"keySet().retainAll(new Set<String>{'user'})", "beforeSize"},
				} {
					fmt.Fprintf(&methods, "public static void mutate%d(Map<String,Schema.SObjectType> m) {m.%s;}\n", id, write.mutation)
					for _, target := range []string{"holder", "alias", "DescribeOwner.stored", "instance.member", "parameter"} {
						action := target + "." + write.mutation + ";"
						if target == "parameter" {
							action = fmt.Sprintf("DescribeOwner.mutate%d(holder);", id)
						}
						body := strings.ReplaceAll(strings.ReplaceAll(string(raw), "@@WRITE@@", action), "@@SIZE@@", write.size)
						// Every method dirties a writable token at the end. Its siblings and
						// subsequent runs must start with the baseline Account token again.
						fmt.Fprintf(&methods, "@IsTest static void path%d_%s() {%s Schema.SObjectType token = sibling.get('account'); token.object='User';}\n", id, strings.ReplaceAll(target, ".", "_"), body)
					}
					id++
				}
				classes := filepath.Join(root, "force-app/main/default/classes")
				writeFile(t, filepath.Join(classes, "DescribeOwner.cls"), "@IsTest public class DescribeOwner {"+methods.String()+"}")
				writeFile(t, filepath.Join(classes, "DescribeOwner.cls-meta.xml"), "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
				index := loadTestIndex(t, root)
				for _, parallel := range []bool{false, true} {
					for range 2 {
						finished := make(chan testreport.Run, 1)
						go func() { finished <- Run(index, Options{Parallelism: 2, ParallelMethods: parallel, NoDiskCache: true}) }()
						result := <-finished
						if got := result.Summary(); got.Total != id*5 || got.Passed != id*5 {
							for _, suite := range result.Suites {
								for _, row := range suite.Cases {
									if row.Problem != nil {
										t.Errorf("%s: %+v", row.MethodName, *row.Problem)
									}
								}
							}
							t.Fatalf("parallel=%v summary=%+v", parallel, got)
						}
					}
				}
			})
		}
	}
}
