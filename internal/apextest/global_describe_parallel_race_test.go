package apextest

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/testreport"
)

// Parallel test methods each bind the global describe to a differently
// spelled map type and store it in a static. Every method runs in its own
// clone, and the clones share the global describe keys copy-on-write. Under
// -race this failed (and could crash with a concurrent map iteration/write)
// while the typed binding rewrote the shared key map in place.
func TestParallelMethodsRebindGlobalDescribeIntoStatics(t *testing.T) {
	previous := runtime.GOMAXPROCS(4)
	t.Cleanup(func() { runtime.GOMAXPROCS(previous) })
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/DescribeCache.cls"), `public class DescribeCache {
    private static Map<String, Schema.SObjectType> rawDescribe;
    public static Map<String, Schema.SObjectType> raw() {
        if (rawDescribe == null) rawDescribe = Schema.getGlobalDescribe();
        return rawDescribe;
    }
    public static Integer count(Map<String, SObjectType> plain) { return plain.size(); }
}`)
	var methods strings.Builder
	for i := range 8 {
		fmt.Fprintf(&methods, `@IsTest static void rebind%d() {
    Map<String, Schema.SObjectType> qualified = DescribeCache.raw();
    Integer size = qualified.size();
    for (Integer i = 0; i < 100; i++) {
        Map<String, SObjectType> plain = qualified;
        unqualified = plain;
        typed = qualified;
        System.assertEquals(size, DescribeCache.count(qualified));
    }
    System.assertEquals(Account.SObjectType, unqualified.get('account'));
}
`, i)
	}
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/DescribeRebindTest.cls"), `@IsTest private class DescribeRebindTest {
static Map<String, Schema.SObjectType> typed;
static Map<String, SObjectType> unqualified;
`+methods.String()+"}")
	index := loadTestIndex(t, root)
	run := Run(index, Options{Parallelism: 8, ParallelMethods: true, NoDiskCache: true})
	if summary := run.Summary(); summary.Total != 8 || summary.Passed != 8 {
		for _, suite := range run.Suites {
			for _, c := range suite.Cases {
				if c.Status != testreport.StatusPass {
					t.Logf("%s: %s %s %v", c.MethodName, c.Status, c.Reason, c.Problem)
				}
			}
		}
		t.Fatalf("summary=%+v", summary)
	}
}
