package apextest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
)

// requestReadFixture is a deterministic small project driven through the
// daemon-shaped request path: RunCasesContext with an attached build
// artifact, the exact options testdaemon.optionsForGeneration builds.
type requestReadFixture struct {
	root     string
	sources  []string
	sidecars []string
	contents map[string]string
	index    typesys.Index
	options  Options
	cases    []TestCase
}

func writeRequestReadFixture(t *testing.T, serviceCount int) *requestReadFixture {
	t.Helper()
	root := t.TempDir()
	contents := map[string]string{}
	write := func(rel, body string) string {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		contents[path] = body
		return path
	}
	write("sfdx-project.json", `{"sourceApiVersion":"62.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	var sources, sidecars []string
	for i := range serviceCount {
		name := fmt.Sprintf("RequestService%02d", i)
		sources = append(sources, write(filepath.Join("force-app/main/default/classes", name+".cls"),
			fmt.Sprintf("public class %s { public static Integer value() { return %d; } }", name, i)))
		sidecar := fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>`)
		sidecars = append(sidecars, write(filepath.Join("force-app/main/default/classes", name+".cls-meta.xml"), sidecar))
	}
	test := `@IsTest private class RequestReadsTest { @IsTest static void passes() { System.assertEquals(0, RequestService00.value()); } }`
	sources = append(sources, write("force-app/main/default/classes/RequestReadsTest.cls", test))
	sidecars = append(sidecars, write("force-app/main/default/classes/RequestReadsTest.cls-meta.xml",
		`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>`))
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := gladeschema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	index, artifacts := typesys.BuildWithArtifacts(p, s)
	for _, diag := range index.Diagnostics {
		if diag.Severity == diagnostic.Error {
			t.Fatalf("fixture diagnostics: %#v", index.Diagnostics)
		}
	}
	options := Options{
		NoDiskCache:     true,
		Parallelism:     1,
		BuildArtifacts:  &artifacts,
		SourceDigests:   artifacts.SourceDigests,
		ProjectSnapshot: &p,
	}
	return &requestReadFixture{
		root:     root,
		sources:  sources,
		sidecars: sidecars,
		contents: contents,
		index:    index,
		options:  options,
		cases:    Discover(index, Options{}),
	}
}

// rewriteIdentical rewrites every fixture file with its current bytes. Every
// stamp changes, so each file is racily clean for the next request while the
// captured generation still matches the content.
func (f *requestReadFixture) rewriteIdentical(t *testing.T) {
	t.Helper()
	for path, body := range f.contents {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// countReads observes validation reads of every fixture path.
func (f *requestReadFixture) countReads(t *testing.T) (reads map[string]int, stop func()) {
	t.Helper()
	var mu sync.Mutex
	reads = make(map[string]int, len(f.sources)+len(f.sidecars))
	stop = SetFileDigestReadHookForTesting(func(path string) {
		mu.Lock()
		reads[path]++
		mu.Unlock()
	})
	return reads, stop
}

func (f *requestReadFixture) assertReadOnce(t *testing.T, reads map[string]int) {
	t.Helper()
	var unexpected []string
	for _, group := range []struct {
		kind  string
		paths []string
	}{
		{"source", f.sources},
		{"companion metadata", f.sidecars},
	} {
		for _, path := range group.paths {
			if got := reads[path]; got != 1 {
				unexpected = append(unexpected, fmt.Sprintf("%s %s: %d reads", group.kind, filepath.Base(path), got))
			}
		}
	}
	if len(unexpected) != 0 {
		t.Fatalf("one warm request read and hashed files more than once, want exactly 1 read per file:\n%s", strings.Join(unexpected, "\n"))
	}
}

// TestWarmRequestReadsAndHashesEachSourceOnce proves one daemon-shaped request
// reads and hashes every captured source and companion metadata file exactly
// once, reusing that digest at every later validation boundary of the same
// request. Files are first rewritten with identical bytes, so each is racily
// clean and no boundary may shortcut on a trusted old stamp: the request
// right after a save is the measured shape.
func TestWarmRequestReadsAndHashesEachSourceOnce(t *testing.T) {
	t.Cleanup(InvalidateRuntimeCaches)
	t.Cleanup(resetFileDigestTrust)
	resetFileDigestTrust()
	f := writeRequestReadFixture(t, 4)
	prime := RunCasesContext(context.Background(), f.index, f.options, f.cases)
	primeSummary := prime.Summary()
	if primeSummary.Total != 1 || primeSummary.Passed != 1 || !primeSummary.OK() {
		t.Fatalf("prime run summary = %#v; suites = %#v", primeSummary, prime.Suites)
	}
	for range 2 {
		f.rewriteIdentical(t)
		reads, stop := f.countReads(t)
		run := RunCasesContext(context.Background(), f.index, f.options, f.cases)
		stop()
		f.assertReadOnce(t, reads)
		if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 || !summary.OK() {
			t.Fatalf("warm run summary = %#v, want the prime run's %#v", summary, primeSummary)
		}
	}
}

// TestWarmRequestStillDetectsEditBetweenValidations keeps the request's
// mismatch guarantee under within-request digest reuse: a save that lands
// between two validation boundaries must still fail the request, exactly as
// it does when every boundary re-reads the file.
func TestWarmRequestStillDetectsEditBetweenValidations(t *testing.T) {
	t.Cleanup(InvalidateRuntimeCaches)
	t.Cleanup(resetFileDigestTrust)
	resetFileDigestTrust()
	f := writeRequestReadFixture(t, 4)
	if prime := RunCasesContext(context.Background(), f.index, f.options, f.cases); !prime.Summary().OK() {
		t.Fatalf("prime run summary = %#v", prime.Summary())
	}
	f.rewriteIdentical(t)
	mutated := false
	target := f.sources[0]
	stop := setSnapshotValidationHookForTesting(func(stage string) {
		if stage == "runtime_after_initial_validation" && !mutated {
			mutated = true
			if err := os.WriteFile(target, []byte("public class RequestService00 { public Integer value() { return 7; } }"), 0o644); err != nil {
				t.Error(err)
			}
		}
	})
	defer stop()
	run := RunCasesContext(context.Background(), f.index, f.options, f.cases)
	stop()
	if !mutated {
		t.Fatal("request never reached runtime_after_initial_validation")
	}
	if summary := run.Summary(); summary.CompileErrors == 0 {
		t.Fatalf("run accepted a source saved between validation boundaries: %#v", summary)
	}
	found := false
	for _, suite := range run.Suites {
		for _, testCase := range suite.Cases {
			if testCase.Problem == nil {
				continue
			}
			if strings.Contains(testCase.Problem.Message, "source snapshot mismatch for") && strings.Contains(testCase.Problem.Message, filepath.ToSlash(target)) {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("no case reported the mismatch for %s; run = %#v", target, testreportRunDetails(run))
	}
}

func testreportRunDetails(run testreport.Run) string {
	var out strings.Builder
	for _, suite := range run.Suites {
		for _, testCase := range suite.Cases {
			if testCase.Problem == nil {
				fmt.Fprintf(&out, "%s.%s %s\n", suite.Name, testCase.MethodName, testCase.Status)
				continue
			}
			fmt.Fprintf(&out, "%s.%s %s: %s\n", suite.Name, testCase.MethodName, testCase.Status, testCase.Problem.Message)
		}
	}
	return out.String()
}
