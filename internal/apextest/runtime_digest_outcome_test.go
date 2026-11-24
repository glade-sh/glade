package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/startupcache"
	"github.com/glade-sh/glade/internal/typesys"
)

// A complete, disk-backed named test run with exact source digests. Each
// service contributes ordinary methods and one assertion in the test suite.
func digestProject(tb testing.TB, services int, rejected bool) (typesys.Index, typesys.BuildArtifacts) {
	tb.Helper()
	root := tb.TempDir()
	classes := filepath.Join(root, "force-app", "main", "default", "classes")
	if err := os.MkdirAll(classes, 0700); err != nil {
		tb.Fatal(err)
	}
	write := func(path, text string) {
		tb.Helper()
		if err := os.WriteFile(path, []byte(text), 0600); err != nil {
			tb.Fatal(err)
		}
	}
	write(filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	var tests strings.Builder
	tests.WriteString("@IsTest private class DigestRunTest {\n")
	for i := range services {
		name := fmt.Sprintf("DigestService%d", i)
		var service strings.Builder
		fmt.Fprintf(&service, "public class %s {\n", name)
		for j := range 8 {
			fmt.Fprintf(&service, "public static Integer offset%d(Integer a){return a + %d;}\n", j, j)
		}
		service.WriteString("}\n")
		write(filepath.Join(classes, name+".cls"), service.String())
		fmt.Fprintf(&tests, "@IsTest static void test%d(){System.assertEquals(8,%s.offset7(1));}\n", i, name)
	}
	if rejected {
		tests.WriteString("@IsTest static void rejected(){MissingDigestNeighbor o;}\n")
	}
	tests.WriteString("}\n")
	write(filepath.Join(classes, "DigestRunTest.cls"), tests.String())
	p, err := project.Load(root)
	if err != nil {
		tb.Fatal(err)
	}
	index, artifacts := typesys.BuildWithArtifacts(p, gladeschema.Schema{})
	if index.HasErrors() {
		tb.Fatal(index.Diagnostics)
	}
	return index, artifacts
}

func TestExactDigestRunnerNeighborOutcome(t *testing.T) {
	t.Cleanup(EnableDiskCacheForTesting())
	t.Cleanup(InvalidateRuntimeCaches)
	for _, rejected := range []bool{false, true} {
		index, artifacts := digestProject(t, 2, rejected)
		run := Run(index, Options{SourceDigests: artifacts.SourceDigests})
		summary := run.Summary()
		if rejected && summary.CompileErrors != 3 || !rejected && summary.Passed != 2 {
			t.Fatalf("rejected %t: %+v", rejected, summary)
		}
		run.DurationMS = 0
		for i := range run.Suites {
			run.Suites[i].DurationMS = 0
			for j := range run.Suites[i].Cases {
				run.Suites[i].Cases[j].DurationMS = 0
			}
		}
		data, err := json.Marshal(run)
		if err != nil {
			t.Fatal(err)
		}
		data = []byte(strings.ReplaceAll(string(data), index.Project.Root, "<project>"))
		// Same fixture on the sequential origin/main control. Durations and
		// the temporary project root are the only normalized fields.
		want := "2df387055d80d65136a5d2370df5c808dff3fbf1bb08355df4b695f1bf329c05"
		if rejected {
			want = "49ebfbe3a0ea5b3e53e7683a93f25219aaa8f1b8f29526fb1ebdaa3583cb466e"
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != want {
			t.Fatalf("rejected=%t report=%s, want %s: %s", rejected, got, want, data)
		}
	}
}

func BenchmarkExactDigestColdProjectRun(b *testing.B) {
	b.Cleanup(EnableDiskCacheForTesting())
	b.Cleanup(InvalidateRuntimeCaches)
	index, artifacts := digestProject(b, 64, false)
	want := RuntimeContentKey(index, artifacts.SourceDigests)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		b.StopTimer()
		InvalidateRuntimeCaches()
		if err := startupcache.Clear(index.Project.Root, startupcache.SubdirTest); err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
		run := Run(index, Options{SourceDigests: artifacts.SourceDigests})
		b.StopTimer()
		if run.Summary().Passed != 64 {
			b.Fatal(run.Summary())
		}
		if entry, err := startupcache.Read(index.Project.Root, startupcache.SubdirTest); err != nil || entry == nil || entry.RuntimeKey != want {
			b.Fatalf("exact-key publication: %v", err)
		}
	}
}
