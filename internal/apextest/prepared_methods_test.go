package apextest

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/testreport"
)

func BenchmarkWideTestClass(b *testing.B) {
	root := b.TempDir()
	writeBenchmarkTestFile(b, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	var source strings.Builder
	source.WriteString("@IsTest private class WideTest {\n")
	for i := 0; i < 256; i++ {
		fmt.Fprintf(&source, "@IsTest static void test%03d() { System.assertEquals(%d, %d); }\n", i, i, i)
	}
	source.WriteString("}")
	writeBenchmarkTestFile(b, filepath.Join(root, "force-app/main/default/classes/WideTest.cls"), source.String())
	index := benchmarkLoadTestIndex(b, root)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		run := Run(index, Options{NoDiskCache: true})
		if s := run.Summary(); s.Total != 256 || s.Passed != 256 {
			b.Fatalf("summary = %+v", s)
		}
	}
}

func TestWideClassMethodIsolation(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	var source strings.Builder
	source.WriteString(`@IsTest private class WideIsolationTest {
 static Integer value = 0;
 @TestSetup static void setup() { insert new Account(Name='seed'); }
`)
	for i := 0; i < 12; i++ {
		fmt.Fprintf(&source, `@IsTest static void test%02d() {
 System.assertEquals(0, value); value = %d;
 System.assertEquals(1, [SELECT count() FROM Account]);
 insert new Account(Name='added'); System.assertEquals(2, [SELECT count() FROM Account]);
 }
`, i, i+1)
	}
	source.WriteString("@IsTest static void fails() { System.assert(false, 'expected failure'); }\n}")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/WideIsolationTest.cls"), source.String())
	index := loadTestIndex(t, root)
	for _, opts := range []Options{{NoDiskCache: true}, {NoDiskCache: true, Parallelism: 2, ParallelMethods: true}} {
		run := Run(index, opts)
		statuses := runCaseStatuses(run)
		if len(statuses) != 13 || statuses["fails"] != testreport.StatusFail {
			t.Fatalf("statuses = %v", statuses)
		}
		for i := 0; i < 12; i++ {
			name := fmt.Sprintf("test%02d", i)
			if statuses[name] != testreport.StatusPass {
				t.Errorf("%s = %s; problem %s", name, statuses[name], firstRunProblem(run))
			}
		}
	}
}
