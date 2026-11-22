package sema

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func returnTextFixture(tb testing.TB, call string) (typesys.Index, []diagnostic.Diagnostic) {
	tb.Helper()
	root := tb.TempDir()
	ownerPath := filepath.Join(root, "Producer.cls")
	testPath := filepath.Join(root, "ProducerTest.cls")
	owner := "public class Producer { public List<String> value() { return null; } }"
	var test strings.Builder
	test.WriteString("@IsTest private class ProducerTest { @IsTest static void check() {\n")
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&test, "Integer x%d = %d;\n", i, i)
	}
	test.WriteString(call + "\n} }")
	for path, contents := range map[string]string{ownerPath: owner, testPath: test.String()} {
		if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
			tb.Fatal(err)
		}
	}
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{ownerPath, testPath}, SourceAPIVersion: "68.0"}, schema.Schema{})
	var offset int
	found := false
	for _, typ := range index.Types {
		if typ.Name == "Producer" && len(typ.Members) > 0 {
			offset, found = typ.Members[0].Range.Start.Offset, true
		}
	}
	if !found {
		tb.Fatal("producer method missing")
	}
	diagnostics := make([]diagnostic.Diagnostic, 40)
	for i := range diagnostics {
		diagnostics[i] = diagnostic.Diagnostic{File: ownerPath, Code: "GLADESEMA019", NativeMessage: "Illegal conversion from List<String> to List<Target>", Range: &diagnostic.Range{Start: diagnostic.Position{Offset: offset}}}
	}
	return index, diagnostics
}

func TestReturnTextRepeatedDiagnosticsOutcome(t *testing.T) {
	for _, tc := range []struct {
		call    string
		matched bool
	}{{"new Producer().value();", true}, {"new Producer().other();", false}} {
		t.Run(tc.call, func(t *testing.T) {
			index, diagnostics := returnTextFixture(t, tc.call)
			before := append([]diagnostic.Diagnostic(nil), diagnostics...)
			out := capturedReturnDependencyNativeText(index, diagnostics)
			for i, d := range out {
				want := "Illegal conversion from List<String> to List<Target>"
				if tc.matched {
					want = "Dependent class is invalid and needs recompilation:\n Class Producer : " + want
				}
				before[i].NativeMessage = want
				if !reflect.DeepEqual(d, before[i]) {
					t.Fatalf("diagnostic = %#v, want %#v", d, before[i])
				}
			}
		})
	}
}

func BenchmarkReturnTextRepeatedDiagnostics(b *testing.B) {
	index, original := returnTextFixture(b, "new Producer().other();")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		diagnostics := append([]diagnostic.Diagnostic(nil), original...)
		capturedReturnDependencyNativeText(index, diagnostics)
	}
}

func TestReturnTextMemoRejectsChangedSource(t *testing.T) {
	index, original := returnTextFixture(t, "new Producer().value();")
	first := append([]diagnostic.Diagnostic(nil), original...)
	if got := capturedReturnDependencyNativeText(index, first)[0].NativeMessage; !strings.HasPrefix(got, "Dependent class is invalid") {
		t.Fatalf("initial message = %q", got)
	}
	path := filepath.Join(index.Project.Root, "ProducerTest.cls")
	if err := os.WriteFile(path, []byte("@IsTest private class ProducerTest { @IsTest static void check() { new Producer().other(); } }"), 0600); err != nil {
		t.Fatal(err)
	}
	second := append([]diagnostic.Diagnostic(nil), original...)
	if got := capturedReturnDependencyNativeText(index, second)[0].NativeMessage; got != original[0].NativeMessage {
		t.Fatalf("changed source reused a previous answer: %q", got)
	}
}

func TestReturnTextMemoSeparatesTestBodiesAndOwners(t *testing.T) {
	index, original := returnTextFixture(t, "new Producer().value();")
	root := index.Project.Root
	testPath := filepath.Join(root, "ProducerTest.cls")
	otherPath := filepath.Join(root, "OtherProducer.cls")
	if err := os.WriteFile(testPath, []byte(`@IsTest private class ProducerTest {
		@IsTest static void first() { new Producer().other(); }
		@IsTest static void second() { new Producer().value(); }
	}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(otherPath, []byte("public class OtherProducer { public List<String> value() { return null; } }"), 0600); err != nil {
		t.Fatal(err)
	}
	index = typesys.Build(project.Project{Root: root, ApexFiles: []string{original[0].File, otherPath, testPath}, SourceAPIVersion: "68.0"}, schema.Schema{})
	first := original[0]
	second := original[0]
	second.File = otherPath
	for _, typ := range index.Types {
		if typ.Name == "OtherProducer" && len(typ.Members) != 0 {
			second.Range = &diagnostic.Range{Start: diagnostic.Position{Offset: typ.Members[0].Range.Start.Offset}}
		}
	}
	out := capturedReturnDependencyNativeText(index, []diagnostic.Diagnostic{first, second})
	if !strings.HasPrefix(out[0].NativeMessage, "Dependent class is invalid") {
		t.Fatalf("second test body did not match: %q", out[0].NativeMessage)
	}
	if out[1].NativeMessage != second.NativeMessage {
		t.Fatalf("another owner reused the match: %q", out[1].NativeMessage)
	}
}

func TestReturnTextMemoSeparatesAPIGenerations(t *testing.T) {
	index, original := returnTextFixture(t, "new Producer().value();")
	for _, api := range []string{"62.0", "68.0", "62.0"} {
		for i := range index.Types {
			if index.Types[i].IsTest {
				index.Types[i].EffectiveAPIVersion = api
			}
		}
		out := capturedReturnDependencyNativeText(index, append([]diagnostic.Diagnostic(nil), original...))
		for _, d := range out {
			if !strings.HasPrefix(d.NativeMessage, "Dependent class is invalid") {
				t.Fatalf("API %s: native message = %q", api, d.NativeMessage)
			}
		}
	}
}
