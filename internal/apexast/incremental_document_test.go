package apexast

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
)

func TestIncrementalDocumentCache(t *testing.T) {
	p := NewParser()
	defer p.Close()
	key := "first"
	sources := []string{
		"public class Example { void run() {} }",
		"public class Example { void run() { Integer n = 1; } }",
		"public class Example { void run() { Integer n = ; } }",
		"public class Example { void run() { [FIND 'term' IN ALL FIELDS]; } }",
	}
	for i, source := range sources {
		path := fmt.Sprintf("File%d.cls", i)
		got := p.ParseDocument(key, path, source)
		want := p.ParseSource(path, source)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("edit %d differs from fresh parse: %#v / %#v", i, got, want)
		}
		if len(p.docTrees) != 1 {
			t.Fatalf("edit %d retained %d trees", i, len(p.docTrees))
		}
	}
	p.ParseDocument("second", "Other.cls", sources[0])
	if len(p.docTrees) != 2 {
		t.Fatalf("documents retained %d trees", len(p.docTrees))
	}
	p.ForgetDocument(key)
	if len(p.docTrees) != 1 {
		t.Fatalf("forget retained %d trees", len(p.docTrees))
	}
	got := p.ParseDocument(key, "Reopened.cls", sources[1])
	if want := p.ParseSource("Reopened.cls", sources[1]); !reflect.DeepEqual(got, want) {
		t.Fatal("reopened document differs from fresh parse")
	}
	p.Close()
	if len(p.docTrees) != 0 {
		t.Fatal("close retained document trees")
	}
}

func TestIncrementalDocumentConcurrent(t *testing.T) {
	p := NewParser()
	defer p.Close()
	const source = "public class Example { Integer value; }"
	want := p.ParseSource("Example.cls", source)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				got := p.ParseDocument(i%2, "Example.cls", source)
				if !reflect.DeepEqual(got, want) {
					t.Errorf("parse differs from fresh output")
				}
				if j%4 == 0 {
					p.ForgetDocument(i % 2)
				}
			}
		}(i)
	}
	wg.Wait()

	closing := NewParser()
	start := make(chan struct{})
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			for j := 0; j < 10; j++ {
				closing.ParseDocument(i, "Example.cls", source)
			}
		}(i)
	}
	close(start)
	closing.Close()
	wg.Wait()
	if len(closing.docTrees) != 0 {
		t.Fatal("concurrent close retained document trees")
	}
}
