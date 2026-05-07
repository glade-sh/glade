package probe

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseToolingSnippetOutputSuccessShape(t *testing.T) {
	raw := []byte(`{
  "status": 0,
  "result": {
    "success": true,
    "compiled": true,
    "executed": true,
    "logs": "USER_DEBUG|[1]|DEBUG|ok"
  }
}`)
	result, err := ParseToolingSnippetOutput("system-test-short", "stdlib", "System.debug('ok');", "sf", raw)
	if err != nil {
		t.Fatal(err)
	}
	if result.ID != "system-test-short" || result.CLI != "sf" || !result.Success || !result.Compiled || !result.Executed {
		t.Fatalf("result = %#v", result)
	}
	if !result.LogsCaptured || result.RawShape.PayloadKey != "result" {
		t.Fatalf("shape = %#v", result.RawShape)
	}
	if result.Fixture == nil || result.Fixture.CommandKind != "tooling-execute-anonymous" || !result.Fixture.Success {
		t.Fatalf("fixture = %#v", result.Fixture)
	}
}

func TestParseToolingSnippetOutputCompileErrorShape(t *testing.T) {
	raw := []byte(`{
  "status": 1,
  "data": {
    "success": false,
    "compiled": false,
    "executed": false,
    "line": "1",
    "column": "8",
    "compileProblem": "Unexpected token 'bad'."
  }
}`)
	result, err := ParseToolingSnippetOutput("bad-compile", "parser", "Integer bad", "sf", raw)
	if err != nil {
		t.Fatal(err)
	}
	if result.Success || result.Compiled || result.Executed || result.CompileProblem == "" {
		t.Fatalf("result = %#v", result)
	}
	if result.RawShape.PayloadKey != "data" || len(result.Diagnostics) != 1 || result.Diagnostics[0].Category != "compile" {
		t.Fatalf("diagnostics/shape = %#v %#v", result.Diagnostics, result.RawShape)
	}
}

func TestParseToolingSnippetOutputRuntimeExceptionShape(t *testing.T) {
	raw := []byte(`{
  "status": 1,
  "result": {
    "success": false,
    "compiled": true,
    "executed": true,
    "exceptionMessage": "System.AssertException: Assertion Failed: forced"
  }
}`)
	result, err := ParseToolingSnippetOutput("assert-fail", "stdlib", "System.assert(false, 'forced');", "sf", raw)
	if err != nil {
		t.Fatal(err)
	}
	if result.Success || !result.Compiled || !result.Executed {
		t.Fatalf("result = %#v", result)
	}
	if result.ExceptionType != "System.AssertException" || len(result.Diagnostics) != 1 || result.Diagnostics[0].Category != "runtime" {
		t.Fatalf("exception/diagnostics = %q %#v", result.ExceptionType, result.Diagnostics)
	}
}

func TestSnippetSourceRejectsAmbiguousInput(t *testing.T) {
	_, err := snippetSource(ToolingSnippet{ID: "ambiguous", Source: "System.debug('x');", File: "snippet.apex"})
	if err == nil {
		t.Fatal("expected ambiguous source/file error")
	}
}

func TestReadToolingSnippetManifestWrappedAndArray(t *testing.T) {
	dir := t.TempDir()
	wrapped := filepath.Join(dir, "wrapped.json")
	if err := os.WriteFile(wrapped, []byte(`{"snippets":[{"id":"one","source":"System.debug('one');"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	snippets, err := ReadToolingSnippetManifest(wrapped)
	if err != nil {
		t.Fatal(err)
	}
	if len(snippets) != 1 || snippets[0].ID != "one" {
		t.Fatalf("wrapped snippets = %#v", snippets)
	}

	array := filepath.Join(dir, "array.json")
	if err := os.WriteFile(array, []byte(`[{"id":"two","source":"System.debug('two');"}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	snippets, err = ReadToolingSnippetManifest(array)
	if err != nil {
		t.Fatal(err)
	}
	if len(snippets) != 1 || snippets[0].ID != "two" {
		t.Fatalf("array snippets = %#v", snippets)
	}
}
