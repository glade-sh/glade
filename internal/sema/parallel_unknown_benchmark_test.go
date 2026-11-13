package sema

import (
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// One unknown assignment among many ordinary method bodies exercises the
// discarded-parallel-pass cost that a large otherwise valid project incurs.
func BenchmarkParallelUnknownAssignment(b *testing.B) {
	root := b.TempDir()
	var files []string
	for i := 0; i < 200; i++ {
		name := fmt.Sprintf("Service%04d", i)
		missing := ""
		if i == 199 {
			missing = "unknownTarget = 1;"
		}
		source := fmt.Sprintf(`public class %s {
            public Integer run(Integer value) {
                Integer total = value;
                for (Integer i = 0; i < 10; i++) { total = total + i; }
                %s
                return total;
            }
        }`, name, missing)
		file := filepath.Join(root, name+".cls")
		writeApexBenchmarkFile(b, file, source)
		files = append(files, file)
	}
	index := typesys.Build(project.Project{Root: root, ApexFiles: files}, schema.Schema{})
	want := NewAnalyzer().checkMethodBodiesWithViewWorkers(index, buildSemaTypeMemberState(index, nil).view(), nil, 1)
	if len(want) == 0 {
		b.Fatal("fixture did not produce an unknown assignment diagnostic")
	}
	a := NewAnalyzer()
	b.ReportAllocs()
	b.ResetTimer()
	var got = want
	for i := 0; i < b.N; i++ {
		got = a.checkMethodBodies(index)
	}
	b.StopTimer()
	if !reflect.DeepEqual(got, want) {
		b.Fatalf("diagnostics differ: serial=%v parallel=%v", want, got)
	}
}
