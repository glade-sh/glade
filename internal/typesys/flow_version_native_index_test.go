package typesys

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
)

// The native rows establish flow invocation behavior. This test covers only
// the index shape: versions of one flow contribute a single generated type.
func TestNativeFlowVersionIndexShape(t *testing.T) {
	root := filepath.Join("testdata", "road255_flow")
	f, err := os.Open(filepath.Join(root, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		t.Run(row[0], func(t *testing.T) {
			v := strings.TrimSuffix(row[1], ".0")
			name := "RoadFlow" + v
			var flows []string
			for _, phase := range strings.Split(row[3], ",") {
				if phase != "missing" {
					flows = append(flows, filepath.Join(root, v, phase, name+".flow-meta.xml"))
				}
			}
			idx := Build(project.Project{Root: t.TempDir(), SourceAPIVersion: row[1], FlowFiles: flows}, schema.Schema{})
			count := 0
			for _, typ := range idx.Types {
				if typ.Name == "Flow.Interview."+name {
					count++
					if typ.SuperClass != "Flow.Interview" || typ.HasSourceSnapshot() {
						t.Fatalf("unexpected generated type: %#v", typ)
					}
				}
			}
			want := 1
			if row[3] == "missing" {
				want = 0
			}
			if count != want || idx.HasErrors() {
				t.Fatalf("generated type count = %d, want %d; diagnostics = %v", count, want, idx.Diagnostics)
			}
		})
	}
}
