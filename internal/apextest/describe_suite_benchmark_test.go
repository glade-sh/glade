package apextest

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// BenchmarkDescribeCloneSuite exercises describe-heavy test isolation with
// owned source. It is a synthetic workload, not a corpus equivalence gate.
func BenchmarkDescribeCloneSuite(b *testing.B) {
	root := b.TempDir()
	writeBenchmarkTestFile(b, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	writeClass := func(name, source string) {
		path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
		writeBenchmarkTestFile(b, path, source)
		writeBenchmarkTestFile(b, path+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>`)
	}
	for i := range 85 {
		name := fmt.Sprintf("DescribeHelper%02d", i)
		writeClass(name, fmt.Sprintf(`public class %s {
  public static Map<String,Schema.SObjectType> types = Schema.getGlobalDescribe();
  public static void observe() {
    for (String name : new List<String>{'Account','Contact','Opportunity','Task','User'}) {
      Schema.DescribeSObjectResult d = types.get(name).getDescribe();
      System.assertEquals(name, d.getName());
      Map<String,Schema.SObjectField> fields = d.fields.getMap();
      System.assert(fields.containsKey('Id'));
      System.assertEquals('Id', fields.get('Id').getDescribe().getName());
    }
    Map<String,List<String>> source = new Map<String,List<String>>{'items'=>new List<String>{'one','two'}};
    Map<String,Object> view = source;
    System.assertEquals(1, view.size());
  }
}`, name))
	}
	for class := range 49 {
		name := fmt.Sprintf("DescribeSuite%02dTest", class)
		var source strings.Builder
		fmt.Fprintf(&source, "@IsTest private class %s {\n", name)
		methods := 15
		if class < 26 {
			methods++
		}
		for method := range methods {
			fmt.Fprintf(&source, "@IsTest static void observe%02d() { DescribeHelper%02d.observe(); }\n", method, (class*16+method)%85)
		}
		source.WriteString("}\n")
		writeClass(name, source.String())
	}
	index := benchmarkLoadTestIndex(b, root)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		run := Run(index, Options{NoDiskCache: true, Parallelism: 2})
		summary := run.Summary()
		if summary.Total != 761 || summary.Passed != 761 {
			b.Fatalf("summary = %#v; suites = %#v", summary, run.Suites[:1])
		}
	}
}
