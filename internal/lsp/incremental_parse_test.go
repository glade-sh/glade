package lsp

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
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

func TestIncrementalDocumentConcurrent(t *testing.T) {
	h := NewHandler(typesys.Index{})
	defer h.parser.Close()
	uri := DocumentURI("file:///Example.cls")
	var workers sync.WaitGroup
	for i := 0; i < 4; i++ {
		workers.Add(1)
		go func(i int) {
			defer workers.Done()
			for j := 0; j < 15; j++ {
				source := fmt.Sprintf("public class Example { Integer f%d = %d; }", i, j)
				doc := h.applyOpen(DidOpenTextDocumentParams{TextDocument: TextDocumentItem{URI: uri, Version: j, Text: source}})
				got, ok := h.parseDocument(doc)
				want := apexast.ParseSource(doc.Path, source).Diagnostics
				if !ok || !reflect.DeepEqual(got, want) {
					t.Errorf("concurrent document differs from fresh parse")
				}
				doc, err := h.applyChange(DidChangeTextDocumentParams{TextDocument: VersionedTextDocumentIdentifier{URI: uri, Version: j + 1}, ContentChanges: []TextDocumentContentChangeEvent{{Text: source + "\n"}}})
				if err != nil {
					t.Errorf("concurrent edit: %v", err)
					return
				}
				got, ok = h.parseDocument(doc)
				want = apexast.ParseSource(doc.Path, doc.Text).Diagnostics
				if !ok || !reflect.DeepEqual(got, want) {
					t.Errorf("concurrent edited document differs from fresh parse")
				}
				h.applyClose(uri)
			}
		}(i)
	}
	workers.Wait()
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

func TestIncrementalDocumentLifecycle(t *testing.T) {
	h := NewHandler(typesys.Index{})
	defer h.parser.Close()
	uri := DocumentURI("file:///Example.cls")
	otherURI := DocumentURI("file:///Other.cls")
	doc := h.applyOpen(DidOpenTextDocumentParams{TextDocument: TextDocumentItem{URI: uri, Version: 1, Text: "public class Example { String s = '😀'; }"}})
	other := h.applyOpen(DidOpenTextDocumentParams{TextDocument: TextDocumentItem{URI: otherURI, Version: 1, Text: "public class Other {}"}})
	check := func(doc openDocument) {
		t.Helper()
		got, ok := h.parseDocument(doc)
		if !ok {
			t.Fatal("parser closed")
		}
		want := h.parser.ParseSource(doc.Path, doc.Text).Diagnostics
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("parse differs from fresh: %#v / %#v", got, want)
		}
	}
	check(doc)
	check(other)
	var err error
	doc, err = h.applyChange(DidChangeTextDocumentParams{TextDocument: VersionedTextDocumentIdentifier{URI: uri, Version: 2}, ContentChanges: []TextDocumentContentChangeEvent{
		{Range: &Range{Start: Position{Character: 35}, End: Position{Character: 37}}, Text: "😃"},
		{Range: &Range{Start: Position{Character: 0}, End: Position{Character: 0}}, Text: "\n"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	check(doc)
	check(other)
	if _, err := h.applyChange(DidChangeTextDocumentParams{TextDocument: VersionedTextDocumentIdentifier{URI: uri, Version: 3}, ContentChanges: []TextDocumentContentChangeEvent{{Range: &Range{Start: Position{Line: 99}, End: Position{Line: 99}}, Text: "x"}}}); err == nil {
		t.Fatal("invalid edit accepted")
	}
	check(doc)
	big, err := h.applyChange(DidChangeTextDocumentParams{TextDocument: VersionedTextDocumentIdentifier{URI: uri, Version: 4}, ContentChanges: []TextDocumentContentChangeEvent{{Text: doc.Text + "\n//" + strings.Repeat("x", maxOverlayDocumentBytes)}}})
	if err != nil {
		t.Fatal(err)
	}
	check(big)
	restored, err := h.applyChange(DidChangeTextDocumentParams{TextDocument: VersionedTextDocumentIdentifier{URI: uri, Version: 5}, ContentChanges: []TextDocumentContentChangeEvent{{Text: doc.Text}}})
	if err != nil {
		t.Fatal(err)
	}
	check(restored)
	h.applyClose(uri)
	check(doc) // stale snapshots take the fresh path
	check(other)
	reopened := h.applyOpen(DidOpenTextDocumentParams{TextDocument: TextDocumentItem{URI: uri, Version: 1, Text: "public class Reopened {}"}})
	check(reopened)
}
