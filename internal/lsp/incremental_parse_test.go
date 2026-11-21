package lsp

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

func TestIncrementalDocumentIdentity(t *testing.T) {
	h := NewHandler(typesys.Index{})
	defer h.parser.Close()
	uri := DocumentURI("file:///Example.cls")
	sources := []string{
		"public class Example { void run() { Integer n = 1; } } @",
		"public class Example { void run() { Integer n = 10 + 2; } } @",
		"public class Example {\r\n void run() { String s = '😀'; }\r\n} @",
		"public class Example { void run() { [FIND 'word' IN ALL FIELDS]; } } @",
		"public class Example { void run() { Integer n = ; } } @",
		"public class Example { void run() { Integer n = 1; } } @",
	}
	var first openDocument
	for i, source := range sources {
		var doc openDocument
		if i == 0 {
			doc = h.applyOpen(DidOpenTextDocumentParams{TextDocument: TextDocumentItem{URI: uri, Version: 1, Text: source}})
			first = doc
		} else {
			var err error
			doc, err = h.applyChange(DidChangeTextDocumentParams{TextDocument: VersionedTextDocumentIdentifier{URI: uri, Version: i + 1}, ContentChanges: []TextDocumentContentChangeEvent{{Text: source}}})
			if err != nil {
				t.Fatal(err)
			}
		}
		fresh := NewHandler(typesys.Index{})
		expected := fresh.applyOpen(DidOpenTextDocumentParams{TextDocument: TextDocumentItem{URI: uri, Version: i + 1, Text: source}})
		want := fresh.documentDiagnostics(expected)
		fresh.parser.Close()
		if got := h.documentDiagnostics(doc); !reflect.DeepEqual(got, want) {
			t.Fatalf("edit %d: got %#v, want %#v", i, got, want)
		}
	}
	// A delayed diagnostics job may still hold an older document snapshot.
	fresh := NewHandler(typesys.Index{})
	defer fresh.parser.Close()
	old := fresh.applyOpen(DidOpenTextDocumentParams{TextDocument: TextDocumentItem{URI: uri, Version: 1, Text: first.Text}})
	if got, want := h.documentDiagnostics(first), fresh.documentDiagnostics(old); !reflect.DeepEqual(got, want) {
		t.Fatalf("stale document: got %#v, want %#v", got, want)
	}
	h.applyClose(uri)
	reopened := h.applyOpen(DidOpenTextDocumentParams{TextDocument: TextDocumentItem{URI: uri, Version: 1, Text: sources[0]}})
	if got, want := h.documentDiagnostics(reopened), fresh.documentDiagnostics(old); !reflect.DeepEqual(got, want) {
		t.Fatalf("reopened document: got %#v, want %#v", got, want)
	}
}

func BenchmarkIncrementalDocument(b *testing.B) {
	var builder strings.Builder
	builder.WriteString("public class Example {\n")
	for i := 0; i < 2048; i++ {
		fmt.Fprintf(&builder, "void method%d() { Integer value = 1; }\n", i)
	}
	builder.WriteString("} @")
	a := builder.String()
	btext := strings.Replace(a, "method1024() { Integer value = 1;", "method1024() { Integer value = 1 + 2;", 1)
	h := NewHandler(typesys.Index{})
	defer h.parser.Close()
	uri := DocumentURI("file:///Example.cls")
	doc := h.applyOpen(DidOpenTextDocumentParams{TextDocument: TextDocumentItem{URI: uri, Version: 1, Text: a}})
	h.documentDiagnostics(doc)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		text := a
		if i%2 == 0 {
			text = btext
		}
		doc, err := h.applyChange(DidChangeTextDocumentParams{TextDocument: VersionedTextDocumentIdentifier{URI: uri, Version: i + 2}, ContentChanges: []TextDocumentContentChangeEvent{{Text: text}}})
		if err != nil {
			b.Fatal(err)
		}
		if len(h.documentDiagnostics(doc)) == 0 {
			b.Fatal("missing diagnostics")
		}
	}
}
