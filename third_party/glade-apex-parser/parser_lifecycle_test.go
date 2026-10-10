//go:build cgo

package apexast

import (
	"reflect"
	"runtime"
	"sync"
	"testing"
	"time"
)

func waitForNoOpenParsers(t *testing.T) {
	t.Helper()
	for range 20 {
		runtime.GC()
		if OpenParsersForTesting() == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("native parsers were not reclaimed: %d remain", OpenParsersForTesting())
}

func TestParserClose(t *testing.T) {
	waitForNoOpenParsers(t)
	(*Parser)(nil).Close()
	p := NewParser()
	defer p.Close()
	if got := OpenParsersForTesting(); got != 1 {
		t.Fatalf("NewParser open count = %d, want 1", got)
	}
	p.ParseSource("Close.cls", "class Close {}")
	p.Close()
	p.Close()
	if got := OpenParsersForTesting(); got != 0 {
		t.Fatalf("Close open count = %d, want 0", got)
	}
	for _, diagnostics := range [][]Diagnostic{
		p.ParseSource("Close.cls", "class Close {}").Diagnostics,
		p.ParseSourceAST("Close.cls", "class Close {}").Diagnostics,
	} {
		if len(diagnostics) != 1 || diagnostics[0].Code != "APEXPARSE001" || diagnostics[0].Message != "parser is closed" {
			t.Fatalf("closed parser diagnostics = %#v", diagnostics)
		}
	}
	if got := OpenParsersForTesting(); got != 0 {
		t.Fatalf("parse reopened a closed parser: %d", got)
	}
}

func TestZeroValueParserLifecycle(t *testing.T) {
	waitForNoOpenParsers(t)
	var p Parser
	defer p.Close()
	if got := p.ParseSource("Zero.cls", "class Zero {}"); len(got.Diagnostics) != 0 {
		t.Fatalf("zero-value parser diagnostics = %#v", got.Diagnostics)
	}
	if got := OpenParsersForTesting(); got != 1 {
		t.Fatalf("zero-value parser open count = %d, want 1", got)
	}
	p.Close()
	if got := OpenParsersForTesting(); got != 0 {
		t.Fatalf("zero-value parser Close open count = %d, want 0", got)
	}
}

func TestParserFinalizer(t *testing.T) {
	waitForNoOpenParsers(t)
	func() {
		p := NewParser()
		p.ParseSource("Abandoned.cls", "class Abandoned {}")
		if got := OpenParsersForTesting(); got != 1 {
			t.Fatalf("abandoned parser open count = %d, want 1", got)
		}
		runtime.KeepAlive(p)
	}()
	waitForNoOpenParsers(t)
}

func TestPooledParsers(t *testing.T) {
	waitForNoOpenParsers(t)
	sources := []string{
		"",
		"public class Pool { void run() { Integer value = 1; } }",
		"class Pool { void run() { @TestVisible Integer value = 1; } }",
		"class Pool { void run( {",
	}
	for _, source := range sources {
		fresh := NewParser()
		want := fresh.ParseSource("Pool.cls", source)
		wantAST := fresh.ParseSourceAST("Pool.cls", source)
		fresh.Close()
		if got := ParseSource("Pool.cls", source); !reflect.DeepEqual(got, want) {
			t.Fatalf("pooled ParseSource differs for %q: got %#v, want %#v", source, got, want)
		}
		if got := ParseSourceAST("Pool.cls", source); !reflect.DeepEqual(got, wantAST) {
			t.Fatalf("pooled ParseSourceAST differs for %q: got %#v, want %#v", source, got, wantAST)
		}
	}
	for i := range 2000 {
		source := sources[i%len(sources)]
		ParseSource("Pool.cls", source)
		ParseSourceAST("Pool.cls", source)
	}
	limit := int64(runtime.GOMAXPROCS(0)*2 + 4)
	// Race builds deliberately discard some pool entries. Allow their queued
	// finalizers to run before checking the number of retained native parsers.
	for attempts := 0; OpenParsersForTesting() > limit && attempts < 20; attempts++ {
		runtime.GC()
		time.Sleep(10 * time.Millisecond)
	}
	if got := OpenParsersForTesting(); got > limit {
		t.Fatalf("pooled parser count = %d, exceeds %d", got, limit)
	}
	// Once sync.Pool drops its cached entries, their finalizers release C memory.
	waitForNoOpenParsers(t)
}

func TestConcurrentParserClose(t *testing.T) {
	p := NewParser()
	defer p.Close()
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			p.ParseSource("Concurrent.cls", "class Concurrent {}")
			p.ParseSourceAST("Concurrent.cls", "class Concurrent {}")
			p.Close()
		})
	}
	wg.Wait()
}
