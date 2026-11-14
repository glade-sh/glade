package sema

import (
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// The caller supplies a copied project that contains an unresolved assignment.
func BenchmarkProjectUnknownAssignment(b *testing.B) {
	root := os.Getenv("GLADE_SEMA_PERF_PROJECT")
	if root == "" {
		b.Skip("GLADE_SEMA_PERF_PROJECT is not set")
	}
	p, err := project.Load(root)
	if err != nil {
		b.Fatal(err)
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		b.Fatal(err)
	}
	index := typesys.Build(p, s)
	procs := runtime.GOMAXPROCS(1)
	serial := Analyze(index).Diagnostics
	runtime.GOMAXPROCS(procs)
	found := false
	for _, d := range serial {
		if d.Code == "GLADESEMA013" && strings.Contains(d.Message, "unknownTarget") {
			found = true
		}
	}
	if !found {
		b.Fatal("project did not produce the unknown assignment diagnostic")
	}
	normalized := append([]diagnostic.Diagnostic(nil), serial...)
	for i := range normalized {
		normalized[i].File = filepath.Base(normalized[i].File)
	}
	data, err := json.Marshal(normalized)
	if err != nil {
		b.Fatal(err)
	}
	b.Logf("normalized diagnostic SHA256 %x (%d diagnostics)", sha256.Sum256(data), len(serial))
	b.ReportAllocs()
	b.ResetTimer()
	var got []diagnostic.Diagnostic
	for i := 0; i < b.N; i++ {
		got = Analyze(index).Diagnostics
	}
	b.StopTimer()
	if !reflect.DeepEqual(got, serial) {
		b.Fatal("parallel diagnostics differ from serial")
	}
}
