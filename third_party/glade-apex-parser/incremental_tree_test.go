//go:build cgo

package apexast

import (
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

func treeShape(n *sitter.Node, out *strings.Builder) {
	fmt.Fprintf(out, "(%s %t %t %t %t %t %d:%d %v:%v", n.Kind(), n.IsNamed(), n.IsExtra(), n.IsError(), n.IsMissing(), n.HasError(), n.StartByte(), n.EndByte(), n.StartPosition(), n.EndPosition())
	for i := uint(0); i < n.ChildCount(); i++ {
		fmt.Fprintf(out, " %s", n.FieldNameForChild(uint32(i)))
		treeShape(n.Child(i), out)
	}
	out.WriteByte(')')
}

func TestIncrementalTreeWideDocument(t *testing.T) {
	var source strings.Builder
	source.WriteString("public class Example {\n")
	for i := 0; i < 2048; i++ {
		fmt.Fprintf(&source, "void method%d() { Integer value = 1; }\n", i)
	}
	source.WriteString("}")
	a := source.String()
	b := strings.Replace(a, "method1024() { Integer value = 1;", "method1024() { Integer value = 1 + 2;", 1)
	p := NewParser()
	defer p.Close()
	var previous *IncrementalTree
	defer func() {
		if previous != nil {
			previous.Close()
		}
	}()
	for _, text := range []string{a, b, a} {
		got, next := p.ParseSourceIncremental("Example.cls", text, previous)
		if previous != nil {
			previous.Close()
		}
		previous = next
		want, fresh := p.ParseSourceIncremental("Example.cls", text, nil)
		var gotShape, wantShape strings.Builder
		treeShape(next.tree.RootNode(), &gotShape)
		treeShape(fresh.tree.RootNode(), &wantShape)
		fresh.Close()
		if !reflect.DeepEqual(got, want) || gotShape.String() != wantShape.String() {
			t.Fatal("wide edited tree differs from fresh parse")
		}
	}
}

func TestIncrementalTreeEditSequence(t *testing.T) {
	p := NewParser()
	defer p.Close()
	rng := rand.New(rand.NewSource(41))
	const initial = "public class Example { void run() { Integer n = 1; String s = '😀'; } }"
	source := initial
	var previous *IncrementalTree
	defer func() {
		if previous != nil {
			previous.Close()
		}
	}()
	fragments := []string{"", "a", " ", "\n", "\r", "😀", "'", "}", "{", ";", "[FIND 'x']"}
	for i := 0; i < 300; i++ {
		if i%15 == 0 {
			source = initial
		}
		start := rng.Intn(len(source) + 1)
		end := start + rng.Intn(len(source)-start+1)
		if end-start > 4 {
			end = start + 4
		}
		source = source[:start] + fragments[rng.Intn(len(fragments))] + source[end:]
		got, next := p.ParseSourceIncremental("Example.cls", source, previous)
		if previous != nil {
			previous.Close()
		}
		previous = next
		want, fresh := p.ParseSourceIncremental("Example.cls", source, nil)
		var gotShape, wantShape strings.Builder
		treeShape(next.tree.RootNode(), &gotShape)
		treeShape(fresh.tree.RootNode(), &wantShape)
		fresh.Close()
		if !reflect.DeepEqual(got, want) || gotShape.String() != wantShape.String() {
			t.Fatalf("edit %d differs from fresh parse; source %q", i, source)
		}
	}
}

func TestIncrementalTreeMatchesFresh(t *testing.T) {
	p := NewParser()
	defer p.Close()
	sources := []string{
		"public class Example { void run() { Integer n = 1; } }",
		"public class Example { void run() { Integer n = 10 + 2; } }",
		"public class Example {\r\n void run() { String s = '😀'; }\r\n}",
		"public class Example {\r void run() { String s = '😁'; }\r}",
		"public class Example { void run() { [FIND 'word' IN ALL FIELDS]; } }",
		"public class Example { void run() { [SELECT Id FROM Account WHERE Name = :value]; } }",
		"public class Example { void run() { Integer n = ; } }",
		"public class Example { void run() { Integer n = 1; } }",
		"public class Example { void run() { Integer n = 1; } }",
		"",
		"public class Example {",
	}
	var previous *IncrementalTree
	defer func() {
		if previous != nil {
			previous.Close()
		}
	}()
	for i, source := range sources {
		var before strings.Builder
		if previous != nil {
			treeShape(previous.tree.RootNode(), &before)
		}
		got, next := p.ParseSourceIncremental("Example.cls", source, previous)
		if previous != nil {
			var after strings.Builder
			treeShape(previous.tree.RootNode(), &after)
			if before.String() != after.String() {
				t.Fatalf("edit %d mutated previous tree", i)
			}
			previous.Close()
		}
		previous = next
		want, fresh := p.ParseSourceIncremental("Example.cls", source, nil)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("edit %d output differs:\n%#v\n%#v", i, got, want)
		}
		var gotShape, wantShape strings.Builder
		treeShape(next.tree.RootNode(), &gotShape)
		treeShape(fresh.tree.RootNode(), &wantShape)
		fresh.Close()
		if gotShape.String() != wantShape.String() {
			t.Fatalf("edit %d tree differs:\n%s\n%s", i, gotShape.String(), wantShape.String())
		}
	}
	previous.Close()
	got, replacement := p.ParseSourceIncremental("Example.cls", sources[0], previous)
	previous = replacement
	want := p.ParseSource("Example.cls", sources[0])
	if !reflect.DeepEqual(got, want) {
		t.Fatal("closed tree did not fall back to a fresh parse")
	}
}
