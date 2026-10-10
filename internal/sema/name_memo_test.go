package sema

import (
	"strings"
	"sync"
	"testing"
)

func semaNameMemoInputs() []string {
	return []string{
		"", " \t\r\n", "\u2003Account__c\u00a0", "already_lower__c",
		" Account__c ", "pkg.Outer.Inner", "Straße", "İstanbul__c", "Σλ",
		"\xffABC", "A\xffZ", "\xff", "System.String", "system.string",
		"HttpRequest", "httprequest", "System.HttpRequest", "system.httprequest",
		"Schema.SObjectType", "SObjectType", "Database.QueryLocator", "QueryLocator",
		"List<System.String>", "Map<System.String,List<System.HttpRequest>>",
		"System.String[]", "List<>", "public System.String", "request.send()", "a+b",
	}
}

func TestSemaNameMemosMatchUncached(t *testing.T) {
	names := newSemaCanonicalNames(64)
	platform := &semaPlatformNameMemo{limit: 64}
	for _, input := range semaNameMemoInputs() {
		for pass := 0; pass < 2; pass++ {
			want := normalizeNameUncached(input)
			if reference := strings.ToLower(strings.TrimSpace(input)); want != reference {
				t.Fatalf("uncached normalization(%q) = %q, want %q", input, want, reference)
			}
			if got := names.canonical(input); got != want {
				t.Errorf("local normalization(%q), pass %d = %q, want %q", input, pass, got, want)
			}
			if got := normalizeName(input); got != want {
				t.Errorf("normalization(%q), pass %d = %q, want %q", input, pass, got, want)
			}
			alias := semaCanonicalPlatformAliasUncached(input)
			if got := platform.canonicalAlias(input); got != alias {
				t.Errorf("local alias(%q), pass %d = %q, want %q", input, pass, got, alias)
			}
			if got := semaCanonicalPlatformAlias(input); got != alias {
				t.Errorf("alias(%q), pass %d = %q, want %q", input, pass, got, alias)
			}
			known := semaKnownPlatformTypeReceiverUncached(input)
			if got := platform.knownReceiver(input); got != known {
				t.Errorf("local receiver(%q), pass %d = %v, want %v", input, pass, got, known)
			}
			if got := semaKnownPlatformTypeReceiver(input); got != known {
				t.Errorf("receiver(%q), pass %d = %v, want %v", input, pass, got, known)
			}
		}
	}
}

func TestSemaNameMemosPreserveReceiverSpelling(t *testing.T) {
	platform := &semaPlatformNameMemo{limit: 32}
	for _, tc := range []struct {
		input string
		want  bool
	}{
		{"HttpRequest", true}, {"HTTPRequest", true}, {"httprequest", false},
		{"System.HttpRequest", true}, {"system.httprequest", true},
		{" HttpRequest ", true}, {" httprequest ", false},
		{"Database.QueryLocator", true}, {"QueryLocator", true}, {"querylocator", false},
		{"String", false}, {"System.String", false},
	} {
		for pass := 0; pass < 2; pass++ {
			if got := platform.knownReceiver(tc.input); got != tc.want {
				t.Errorf("receiver(%q), pass %d = %v, want %v", tc.input, pass, got, tc.want)
			}
		}
	}
	for _, input := range []string{"HttpRequest", "HTTPRequest", "httprequest", " HttpRequest "} {
		if _, ok := platform.receivers[input]; !ok {
			t.Errorf("receiver memo did not retain exact key %q", input)
		}
	}
}

func TestSemaNameMemosRemainBounded(t *testing.T) {
	names := newSemaCanonicalNames(2)
	platform := &semaPlatformNameMemo{limit: 2}
	for _, input := range []string{"FirstName", "SecondName", "ThirdName"} {
		if got, want := names.canonical(input), normalizeNameUncached(input); got != want {
			t.Errorf("normalization(%q) = %q, want %q", input, got, want)
		}
		if got, want := platform.canonicalAlias(input), semaCanonicalPlatformAliasUncached(input); got != want {
			t.Errorf("alias(%q) = %q, want %q", input, got, want)
		}
		if got, want := platform.knownReceiver(input), semaKnownPlatformTypeReceiverUncached(input); got != want {
			t.Errorf("receiver(%q) = %v, want %v", input, got, want)
		}
	}
	if names.size() != 2 || len(platform.aliases) != 2 || len(platform.receivers) != 2 {
		t.Fatalf("memo sizes = (%d, %d, %d), want (2, 2, 2)", names.size(), len(platform.aliases), len(platform.receivers))
	}
	for _, limit := range []int{0, -1} {
		disabled := &semaPlatformNameMemo{limit: limit}
		if got := disabled.canonicalAlias("System.HttpRequest"); got != "HttpRequest" {
			t.Errorf("disabled alias = %q, want HttpRequest", got)
		}
		if !disabled.knownReceiver("System.HttpRequest") || len(disabled.aliases) != 0 || len(disabled.receivers) != 0 {
			t.Errorf("disabled platform memo did not preserve uncached behavior")
		}
	}
}

func TestSemaNameMemosSkipOversizedInputsAndOutputs(t *testing.T) {
	names := newSemaCanonicalNames(8)
	platform := &semaPlatformNameMemo{limit: 8}
	for _, input := range []string{
		strings.Repeat("A", semaNameMemoMaxBytes+1),
		strings.Repeat("\xff", semaNameMemoMaxBytes+1),
		"System.HttpRequest" + strings.Repeat(" ", semaNameMemoMaxBytes),
	} {
		if got, want := normalizeName(input), normalizeNameUncached(input); got != want {
			t.Errorf("oversized normalization differs")
		}
		if got, want := platform.canonicalAlias(input), semaCanonicalPlatformAliasUncached(input); got != want {
			t.Errorf("oversized alias differs")
		}
		if got, want := platform.knownReceiver(input), semaKnownPlatformTypeReceiverUncached(input); got != want {
			t.Errorf("oversized receiver differs")
		}
	}
	for _, input := range []string{strings.Repeat("A", semaNameMemoMaxBytes+1), strings.Repeat("\xff", semaNameMemoMaxBytes)} {
		if got, want := names.canonical(input), normalizeNameUncached(input); got != want {
			t.Errorf("oversized local normalization differs")
		}
	}
	expanded := "List<" + strings.Repeat("FieldDescribeOptions,", 40) + "FieldDescribeOptions>"
	if got, want := platform.canonicalAlias(expanded), semaCanonicalPlatformAliasUncached(expanded); got != want || len(got) <= semaNameMemoMaxBytes {
		t.Fatalf("expanded alias = %q, want uncached output exceeding byte limit", got)
	}
	if names.size() != 0 || len(platform.aliases) != 0 || len(platform.receivers) != 0 {
		t.Fatalf("oversized entries retained: (%d, %d, %d)", names.size(), len(platform.aliases), len(platform.receivers))
	}
}

func TestSemaNameMemosConcurrentLookups(t *testing.T) {
	names := newSemaCanonicalNames(8)
	platform := &semaPlatformNameMemo{limit: 8}
	inputs := semaNameMemoInputs()
	var workers sync.WaitGroup
	for worker := 0; worker < 16; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for iteration := 0; iteration < 20; iteration++ {
				for _, input := range inputs {
					if got, want := names.canonical(input), normalizeNameUncached(input); got != want {
						t.Errorf("concurrent normalization(%q) = %q, want %q", input, got, want)
						return
					}
					if got, want := platform.canonicalAlias(input), semaCanonicalPlatformAliasUncached(input); got != want {
						t.Errorf("concurrent alias(%q) = %q, want %q", input, got, want)
						return
					}
					if got, want := platform.knownReceiver(input), semaKnownPlatformTypeReceiverUncached(input); got != want {
						t.Errorf("concurrent receiver(%q) = %v, want %v", input, got, want)
						return
					}
				}
			}
		}()
	}
	workers.Wait()
	if names.size() > 8 || len(platform.aliases) > 8 || len(platform.receivers) > 8 {
		t.Fatalf("concurrent memo insertion exceeded limits")
	}
}

func TestSemaNameMemosRepeatedLookupDoesNotAllocate(t *testing.T) {
	names := newSemaCanonicalNames(8)
	platform := &semaPlatformNameMemo{limit: 8}
	const input = "System.HttpRequest"
	names.canonical(input)
	platform.canonicalAlias(input)
	platform.knownReceiver(input)
	for _, tc := range []struct {
		name   string
		lookup func()
	}{
		{"normalization", func() { names.canonical(input) }},
		{"lowercase", func() { normalizeName("already_lower__c") }},
		{"alias", func() { platform.canonicalAlias(input) }},
		{"receiver", func() { platform.knownReceiver(input) }},
	} {
		if allocations := testing.AllocsPerRun(100, tc.lookup); allocations != 0 {
			t.Errorf("cached %s allocations = %.2f, want 0", tc.name, allocations)
		}
	}
}
