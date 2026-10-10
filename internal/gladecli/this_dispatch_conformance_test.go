package gladecli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
)

type ownedThisDispatchSource struct {
	Source       string `json:"source"`
	SourceSHA256 string `json:"sourceSHA256"`
}

type ownedThisDispatchCapture struct {
	Anonymous struct {
		Observations map[string]string       `json:"observations"`
		Runtime      ownedThisDispatchSource `json:"runtime"`
		Rejection    ownedThisDispatchSource `json:"rejection"`
	} `json:"anonymous"`
	IsTest struct {
		Rows []struct {
			ownedThisDispatchSource
			ID, Name, Method, Compilation, Expected, Outcome, Message string
		} `json:"rows"`
	} `json:"isTest"`
}

func ownedThisDispatchSourceText(t *testing.T, source ownedThisDispatchSource) string {
	t.Helper()
	if source.Source == "" || fmt.Sprintf("%x", sha256.Sum256([]byte(source.Source))) != source.SourceSHA256 {
		t.Fatal("this_dispatch source differs from native capture export")
	}
	return source.Source
}

func ownedThisDispatchValidate(t *testing.T, api string, capture ownedThisDispatchCapture) {
	t.Helper()
	if len(capture.Anonymous.Observations) != 5 || len(capture.IsTest.Rows) != 5 {
		t.Fatal("incomplete this_dispatch anonymous/@IsTest matrix")
	}
	seen := make(map[string]bool)
	for _, row := range capture.IsTest.Rows {
		if seen[row.ID] || row.Name == "" {
			t.Fatalf("duplicate/missing this_dispatch named identity: %s", row.ID)
		}
		seen[row.ID] = true
		ownedThisDispatchSourceText(t, row.ownedThisDispatchSource)
		if row.ID == "C001" {
			if row.Method != "" || !strings.HasPrefix(row.Compilation, "COMPILE_ERROR\t") ||
				!strings.HasPrefix(capture.Anonymous.Observations[row.ID], "COMPILE_ERROR\t") {
				t.Fatal("missing native lexical-this compiler rejection")
			}
		} else if row.Compilation != "compiled" || row.Method != row.ID || row.Expected != capture.Anonymous.Observations[row.ID] ||
			row.Outcome != "Fail" || row.Message != "System.AssertException: Assertion Failed: P|"+row.ID+"|"+row.Expected {
			t.Fatalf("incomplete native this_dispatch runtime row: %+v", row)
		}
	}
	for _, id := range []string{"R001", "R002", "R003", "R004", "C001"} {
		if !seen[id] || capture.Anonymous.Observations[id] == "" {
			t.Fatalf("missing native this_dispatch row: %s", id)
		}
	}
}

func ownedThisDispatchAnonymousProject(t *testing.T, api string) string {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
	return root
}

func ownedThisDispatchConformance(t *testing.T, api string, capture ownedThisDispatchCapture) {
	t.Helper()
	ownedThisDispatchValidate(t, api, capture)
	t.Run("anonymous/runtime", func(t *testing.T) {
		source := ownedThisDispatchSourceText(t, capture.Anonymous.Runtime)
		var stdout, stderr bytes.Buffer
		// The captured object graph is cyclic. Read the CLI debug transport
		// without asking its JSON encoder to serialize all execution variables.
		code := Run(context.Background(), []string{"exec", "--project", ownedThisDispatchAnonymousProject(t, api), source}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("exit=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
		}
		seen := make(map[string]bool)
		for _, line := range strings.Split(stdout.String(), "\n") {
			message, ok := strings.CutPrefix(line, "  USER_DEBUG ")
			if !ok {
				continue
			}
			parts := strings.SplitN(message, "|", 3)
			if len(parts) != 3 || parts[0] != "P" || parts[1] == "C001" || seen[parts[1]] {
				t.Fatalf("anonymous observation differs from native this_dispatch: %q", message)
			}
			want, captured := capture.Anonymous.Observations[parts[1]]
			if !captured || want != parts[2] {
				t.Fatalf("anonymous observation differs from native this_dispatch: %q", message)
			}
			seen[parts[1]] = true
		}
		if len(seen) != 4 {
			t.Fatalf("this_dispatch runtime rows=%d, want 4", len(seen))
		}
	})
	for _, row := range capture.IsTest.Rows {
		if row.ID == "C001" {
			continue
		}
		t.Run("isTest/"+row.ID, func(t *testing.T) {
			// Replace only the deliberate transport failure with the exact
			// captured payload assertion; lexical this and case bodies stay intact.
			marker := "System.assert(false,'P|" + row.ID + "|'+String.valueOf(r));"
			source := ownedThisDispatchSourceText(t, row.ownedThisDispatchSource)
			if strings.Count(source, marker) != 1 {
				t.Fatal("missing/repeated native terminal assertion")
			}
			assertion := "System.assertEquals('" + row.Expected + "',String.valueOf(r));"
			source = strings.Replace(source, marker, assertion, 1)
			root := ownedOverloadSourceProject(t, source, row.Name, api)
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), []string{"test", "--project", root, "--class", row.Name, "--no-cache", "--no-serve", "--no-progress", "--json"}, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("exit=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
			}
			run, err := decodeTestRunJSON(stdout.Bytes())
			if err != nil {
				t.Fatal(err)
			}
			count := 0
			for _, suite := range run.Suites {
				for _, result := range suite.Cases {
					count++
					if result.ClassName != row.Name || result.MethodName != row.Method || result.Status != testreport.StatusPass || result.Problem != nil {
						t.Fatalf("native payload %q: unexpected runtime result: %+v", row.Expected, result)
					}
				}
			}
			if count != 1 {
				t.Fatalf("ran %d named methods, want 1", count)
			}
		})
	}
	ownedThisDispatchRejection(t, api, capture)
}

func ownedThisDispatchRejection(t *testing.T, api string, capture ownedThisDispatchCapture) {
	t.Helper()
	ownedThisDispatchValidate(t, api, capture)
	t.Run("anonymous/C001", func(t *testing.T) {
		source := ownedThisDispatchSourceText(t, capture.Anonymous.Rejection)
		root := ownedThisDispatchAnonymousProject(t, api)
		var stdout, stderr bytes.Buffer
		code := Run(context.Background(), []string{"exec", "--project", root, "--json", source}, &stdout, &stderr)
		if code != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "GLADESEMA009:") {
			t.Fatalf("want lexical-this rejection before execution; exit=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
		}
		// C001 is rejected while preparing its transient declarations. Build
		// those same declarations with their original source positions so the
		// Native line prefix is checked independently of CLI error rendering.
		parsed := apexast.NewParser().ParseSource("__glade_anonymous.cls", source)
		var paths []string
		for i, declaration := range parsed.Declarations {
			if declaration.Kind != apexast.DeclarationClass {
				t.Fatalf("unexpected captured declaration: %s", declaration.Kind)
			}
			start, end := declaration.Range.Start.Offset, declaration.Range.End.Offset
			prefix := []byte(source[:start])
			for j := range prefix {
				if prefix[j] != '\n' && prefix[j] != '\r' {
					prefix[j] = ' '
				}
			}
			path := filepath.Join(root, fmt.Sprintf("GladeAnonymous%d.cls", i))
			writeTestFile(t, path, string(prefix)+source[start:end])
			paths = append(paths, path)
		}
		if len(paths) != 4 {
			t.Fatalf("captured anonymous declarations=%d, want 4", len(paths))
		}
		index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: paths}, schema.Schema{})
		if index.HasErrors() {
			t.Fatal(index.Diagnostics)
		}
		index = sema.WithAnonymousDeclarationContext(index)
		analysis := sema.AnalyzeAnonymousDeclarationsInContext(index, index)
		got := ownedOverloadCompileObservation(t, analysis.Diagnostics, "GLADESEMA009", true)
		for _, item := range analysis.Diagnostics {
			if item.Severity == diagnostic.Error && stderr.String() != "glade: "+item.Code+": "+anonymousNativeMessage(item)+"\n" {
				t.Fatalf("anonymous CLI differs from exact native declaration diagnostic: %q", stderr.String())
			}
		}
		if got != capture.Anonymous.Observations["C001"] {
			t.Fatalf("lexical-this rejection=%q native=%q", got, capture.Anonymous.Observations["C001"])
		}
	})
	for _, row := range capture.IsTest.Rows {
		if row.ID != "C001" {
			continue
		}
		t.Run("isTest/C001", func(t *testing.T) {
			root := ownedOverloadSourceProject(t, ownedThisDispatchSourceText(t, row.ownedThisDispatchSource), row.Name, api)
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), []string{"check", "--project", root, "--no-cache", "--no-progress", "--json"}, &stdout, &stderr)
			if code != 1 {
				t.Fatalf("want lexical-this rejection; exit=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
			}
			var envelope struct{ Diagnostics []diagnostic.Diagnostic }
			if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
				t.Fatal(err)
			}
			local := ownedOverloadCompileObservation(t, envelope.Diagnostics, "GLADESEMA009", false)
			index, err := loadIndex(root)
			if err != nil {
				t.Fatal(err)
			}
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			analysis := sema.Analyze(index)
			got := ownedOverloadCompileObservation(t, analysis.Diagnostics, "GLADESEMA009", false)
			for _, item := range analysis.Diagnostics {
				if item.Severity == diagnostic.Error && local != "COMPILE_ERROR\t"+item.Message {
					t.Fatalf("CLI local diagnostic=%q analyzer=%q", local, item.Message)
				}
			}
			if got != row.Compilation {
				t.Fatalf("lexical-this rejection=%q native=%q", got, row.Compilation)
			}
		})
	}
}
