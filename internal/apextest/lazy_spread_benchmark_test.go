package apextest

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// BenchmarkLazySpreadMissingQueries measures repeated query misses in isolated
// test methods and in a bulk insert trigger. Project loading is outside timing.
func BenchmarkLazySpreadMissingQueries(b *testing.B) {
	root := b.TempDir()
	writeBenchmarkTestFile(b, filepath.Join(root, "Neighbor.trigger"), `trigger Neighbor on Account (before insert) {
    for (Account row : Trigger.new) {
        try { Database.query('SELECT Id FROM UncapturedNeighbor__mdt'); }
        catch (Exception e) { row.Description = e.getTypeName(); }
    }
}`)
	var source strings.Builder
	source.WriteString("@IsTest private class NeighborBenchTest {\n")
	for i := range 8 {
		fmt.Fprintf(&source, `@IsTest static void run%d() {
    for (Integer i = 0; i < 20; i++) {
        try { Database.query('SELECT Id FROM UncapturedNeighbor__mdt'); System.assert(false); }
        catch (System.QueryException e) { System.assert(e.getMessage().contains('is not supported')); }
    }
    List<Account> rows = new List<Account>();
    for (Integer i = 0; i < 10; i++) rows.add(new Account(Name='Sprout'));
    insert rows;
    List<Account> saved = [SELECT Description FROM Account];
    System.assertEquals(10, saved.size());
    for (Account row : saved) System.assertEquals('System.QueryException', row.Description);
}
`, i)
	}
	source.WriteString("}\n")
	writeBenchmarkTestFile(b, filepath.Join(root, "NeighborBenchTest.cls"), source.String())
	index := benchmarkLoadTestIndex(b, root)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		report := Run(index, Options{NoDiskCache: true, Parallelism: 1})
		if summary := report.Summary(); summary.Total != 8 || summary.Passed != 8 {
			b.Fatalf("summary = %#v; suites = %#v", summary, report.Suites)
		}
	}
}
