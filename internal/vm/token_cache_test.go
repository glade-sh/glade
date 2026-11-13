package vm

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/glade-sh/glade/internal/ir"
)

func cacheTestBody(n int) string {
	var b strings.Builder
	b.WriteString("Integer total = 0;\n")
	for i := 0; i < 20; i++ {
		fmt.Fprintf(&b, "for (Integer i%d = 0; i%d < %d; i%d++) { total += i%d + %d; } // body %d\n", i, i, i+3, i, i, n, n)
	}
	b.WriteString("String s = 'it''s \\\\ done';\n")
	return b.String()
}

func cacheCorpus(t *testing.T) []string {
	t.Helper()
	bodies := []string{
		"", "x", "Integer a = 1;", "Integer a = ;", "String s = 'open", "/* open",
		"Integer a = 1e5;", "Integer a = 99999999999;", "Integer a = 5 % 2;",
		"try { throw new Exception('x'); } catch (Exception e) { } finally { }",
		"List<Account> l = [SELECT Id FROM Account WHERE CreatedDate < TODAY];",
		"String t = '''\nmulti\nline''';",
	}
	for i := 0; i < 6; i++ {
		bodies = append(bodies, cacheTestBody(i))
	}
	root := filepath.Join("..", "..", "testdata", "local-tests")
	count := 0
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || count >= 150 {
			return nil
		}
		if ext := filepath.Ext(path); ext == ".cls" || ext == ".trigger" {
			if data, err := os.ReadFile(path); err == nil {
				bodies = append(bodies, string(data))
				count++
			}
		}
		return nil
	})
	if count == 0 {
		t.Fatal("no testdata sources found")
	}
	return bodies
}

func TestCompileLexesEachBodyOnce(t *testing.T) {
	resetTokenCache()
	body := cacheTestBody(1000)
	before := lexScanRuns.Load()
	// The semantic check and the runtime compile use different options.
	if _, err := CompileAnonymousWithOptions(body, CompileOptions{PrefixStatementCandidates: true, NoLocalMethods: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: "67.0"}); err != nil {
		t.Fatal(err)
	}
	if _, err := CompileAnonymous(body); err != nil {
		t.Fatal(err)
	}
	if got := lexScanRuns.Load() - before; got != 1 {
		t.Fatalf("lexed %d times, want 1", got)
	}
}

func TestCachedCompileMatchesUncached(t *testing.T) {
	optionSets := []CompileOptions{
		{}, {APIVersion: "62.0"}, {Trigger: true}, {PrefixStatementCandidates: true},
		{NoLocalMethods: true}, {PrefixStatementCandidates: true, NoLocalMethods: true},
	}
	for _, body := range cacheCorpus(t) {
		for _, opts := range optionSets {
			disableTokenCache.Store(true)
			wantProg, wantErr := CompileAnonymousWithOptions(body, opts)
			wantExc, wantFinal := ExceptionSyntaxError(body), FinalCollectionArraySyntaxError(body)
			disableTokenCache.Store(false)
			resetTokenCache()
			for round := 0; round < 2; round++ {
				gotProg, gotErr := CompileAnonymousWithOptions(body, opts)
				if !reflect.DeepEqual(wantProg, gotProg) || !reflect.DeepEqual(wantErr, gotErr) {
					t.Fatalf("compile differs (round %d) for %.60q opts %+v: %v vs %v", round, body, opts, wantErr, gotErr)
				}
				if !reflect.DeepEqual(wantExc, ExceptionSyntaxError(body)) || !reflect.DeepEqual(wantFinal, FinalCollectionArraySyntaxError(body)) {
					t.Fatalf("syntax diagnostics differ for %.60q", body)
				}
			}
		}
	}
}

func TestCachedTokensAreNotMutatedByConsumers(t *testing.T) {
	resetTokenCache()
	for _, body := range cacheCorpus(t) {
		toks, err := lexWithAnnotations(body, false)
		if err != nil {
			continue
		}
		if cap(toks) != len(toks) {
			t.Fatalf("cached slice has spare capacity (%d/%d); an append would write into shared storage", len(toks), cap(toks))
		}
		snapshot := make([]token, len(toks))
		for i, tok := range toks {
			tok.escapedBackslashOffsets = append([]int(nil), tok.escapedBackslashOffsets...)
			snapshot[i] = tok
		}
		for _, opts := range []CompileOptions{{}, {PrefixStatementCandidates: true}, {NoLocalMethods: true, APIVersion: "62.0"}} {
			_, _ = CompileAnonymousWithOptions(body, opts)
		}
		_, _, _ = compileExpressionPrefix(body)
		again, _ := lexWithAnnotations(body, false)
		for i := range snapshot {
			a, b := snapshot[i], again[i]
			if a.kind != b.kind || a.text != b.text || a.pos != b.pos || !reflect.DeepEqual(append([]int(nil), a.escapedBackslashOffsets...), append([]int(nil), b.escapedBackslashOffsets...)) {
				t.Fatalf("token %d mutated for %.60q", i, body)
			}
		}
	}
}

func TestTokenCacheStaysBounded(t *testing.T) {
	resetTokenCache()
	for i := 0; i < 4000; i++ {
		if _, err := lexWithAnnotations(cacheTestBody(i), false); err != nil {
			t.Fatal(err)
		}
	}
	if got := tokenCacheBytes(); got > tokenCacheLimit {
		t.Fatalf("cache holds %d bytes, limit %d", got, tokenCacheLimit)
	}
}

func TestTokenCacheConcurrentCompiles(t *testing.T) {
	resetTokenCache()
	bodies := make([]string, 8)
	want := make([]ir.Program, len(bodies))
	disableTokenCache.Store(true)
	for i := range bodies {
		bodies[i] = cacheTestBody(i)
		prog, _ := CompileAnonymous(bodies[i])
		want[i] = prog
	}
	disableTokenCache.Store(false)
	var wg sync.WaitGroup
	for g := 0; g < 16; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for n := 0; n < 40; n++ {
				i := (g + n) % len(bodies)
				prog, err := CompileAnonymousWithOptions(bodies[i], CompileOptions{PrefixStatementCandidates: g%2 == 0})
				if err != nil {
					t.Error(err)
					return
				}
				if g%2 == 1 && !reflect.DeepEqual(prog, want[i]) {
					t.Errorf("body %d differs under concurrency", i)
					return
				}
			}
		}(g)
	}
	wg.Wait()
}
