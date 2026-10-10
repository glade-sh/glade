package apexast

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestPooledParsersMatchFreshParsers(t *testing.T) {
	for _, root := range []string{"testdata", filepath.Join("..", "..", "testdata")} {
		if _, err := os.Stat(root); root == "testdata" && os.IsNotExist(err) {
			continue // This package currently uses the repository's shared fixtures.
		}
		fixtures := 0
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || filepath.Ext(path) != ".cls" {
				return nil
			}
			fixtures++
			t.Run(filepath.ToSlash(path), func(t *testing.T) {
				source, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				assertPooledParsersMatchFresh(t, path, string(source))
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if fixtures == 0 {
			t.Fatalf("no .cls fixtures found under %s", root)
		}
	}
	for _, tc := range []struct {
		name   string
		source string
	}{
		{"empty", ""},
		{"valid", "public class Example { public static Integer run() { return 1; } }"},
		{"syntax_error", "public class Broken { public void run( {"},
		{"body_annotations", "public class Example { public void run() { @SuppressWarnings('unused') Integer value = 1; } }"},
		{"sosl_diagnostic", "public class Example { public void run() { List<List<SObject>> rows = [FIND 'needle' RETURNING Account(Id WHERE)]; } }"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assertPooledParsersMatchFresh(t, tc.name+".cls", tc.source)
		})
	}
}

func assertPooledParsersMatchFresh(t *testing.T, path, source string) {
	t.Helper()
	fresh := NewParser()
	want := fresh.ParseSource(path, source)
	fresh.Close()
	if got := ParseSource(path, source); !reflect.DeepEqual(got, want) {
		t.Fatalf("pooled ParseSource differs from a fresh parser:\ngot: %#v\nwant: %#v", got, want)
	}
	fresh = NewParser()
	wantAST := fresh.ParseSourceAST(path, source)
	fresh.Close()
	if got := ParseSourceAST(path, source); !reflect.DeepEqual(got, wantAST) {
		t.Fatalf("pooled ParseSourceAST differs from a fresh parser:\ngot: %#v\nwant: %#v", got, wantAST)
	}
}

func TestPooledParsersBoundOpenCount(t *testing.T) {
	waitForOpenParsers(t, 0)
	limit := int64(runtime.GOMAXPROCS(0)*2 + 4)
	for i := range 2000 {
		source := fmt.Sprintf("public class Body%d { public void run() { Integer value = %d; } }", i, i)
		if i%2 == 0 {
			if got := ParseSource("Body.cls", source); len(got.Diagnostics) != 0 {
				t.Fatalf("ParseSource diagnostics: %#v", got.Diagnostics)
			}
		} else if got := ParseSourceAST("Body.cls", source); len(got.Diagnostics) != 0 {
			t.Fatalf("ParseSourceAST diagnostics: %#v", got.Diagnostics)
		}
		if (i+1)%100 == 0 {
			// The race detector deliberately drops some sync.Pool entries.
			// Allow their finalizers to run before checking live native parsers.
			waitForOpenParsers(t, limit)
		}
	}
	if got := OpenParsersForTesting(); got > limit {
		t.Fatalf("open parsers = %d, want <= %d after 2000 pooled parses", got, limit)
	}
}

func TestParserClose(t *testing.T) {
	waitForOpenParsers(t, 0)
	var nilParser *Parser
	nilParser.Close()
	new(Parser).Close()
	parser := NewParser()
	if got := OpenParsersForTesting(); got != 1 {
		t.Fatalf("open parsers = %d after NewParser, want 1", got)
	}
	parser.ParseSource("Example.cls", "public class Example {}")
	var workers sync.WaitGroup
	for range 8 {
		workers.Go(parser.Close)
	}
	workers.Wait()
	parser.Close()
	if got := OpenParsersForTesting(); got != 0 {
		t.Fatalf("open parsers = %d after repeated Close, want 0", got)
	}
}

func TestParserFinalizer(t *testing.T) {
	waitForOpenParsers(t, 0)
	func() {
		parser := NewParser()
		parser.ParseSource("Dropped.cls", "public class Dropped {}")
		if got := OpenParsersForTesting(); got != 1 {
			t.Fatalf("open parsers = %d before dropping parser, want 1", got)
		}
		runtime.KeepAlive(parser)
	}()
	waitForOpenParsers(t, 0)
}

func TestPooledParsersConcurrent(t *testing.T) {
	var workers sync.WaitGroup
	for worker := range 8 {
		workers.Go(func() {
			for iteration := range 25 {
				name := fmt.Sprintf("Concurrent%d_%d", worker, iteration)
				path := name + ".cls"
				source := "public class " + name + " { public Integer value = 1; }"
				fresh := NewParser()
				want := fresh.ParseSource(path, source)
				wantAST := fresh.ParseSourceAST(path, source)
				fresh.Close()
				got := ParseSource(path, source)
				gotAST := ParseSourceAST(path, source)
				// Reuse pooled parsers before comparing to detect returned data
				// accidentally retaining storage owned by the parser.
				ParseSource("Later.cls", strings.Repeat(" ", iteration)+"public class Later {}")
				ParseSourceAST("Later.cls", "public class Later {}")
				if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(gotAST, wantAST) {
					t.Errorf("concurrent pooled parse differs for %s", path)
					return
				}
			}
		})
	}
	workers.Wait()
}

func waitForOpenParsers(t *testing.T, limit int64) {
	t.Helper()
	for range 20 {
		runtime.GC()
		if OpenParsersForTesting() <= limit {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("open parsers = %d after GC/finalizer retries, want <= %d", OpenParsersForTesting(), limit)
}
