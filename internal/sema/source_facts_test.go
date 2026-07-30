package sema

import (
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

func TestSourceFactsPreserveLexicalSemantics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source string
	}{
		{
			name: "nested braces",
			source: `public class Nested {
    void run() {
        if (true) {
            for (Integer i = 0; i < 2; i++) {
            }
        }
    }
}`,
		},
		{
			name: "strings comments and escaped quotes",
			source: `public class Masked {
    String a = '{ not code }';
    String b = "also { not code }";
    String c = 'escaped \' }';
    // { line comment }
    /* { block
       comment } */
    void run() { }
}`,
		},
		{
			name: "doubled Apex quote and queries",
			source: `public class Queries {
    String text = 'it''s still { text }';
    List<Account> rows = [SELECT Id FROM Account WHERE Name = 'A}'];
    List<List<SObject>> found = [FIND 'A{' IN ALL FIELDS RETURNING Account(Id)];
}`,
		},
		{
			name: "annotations and generic syntax",
			source: `@AuraEnabled(cacheable=true)
public class GenericType<T extends List<Map<String, Account>>> {
    @InvocableMethod(label='Run {now}')
    public static List<Account> run(List<Id> ids) { return new List<Account>(); }
}`,
		},
		{
			name: "malformed source",
			source: `public class Broken {
    void run( {
        String value = 'unterminated {';
        /* unterminated comment
}`,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			facts := newSourceFacts(test.source)
			legacySpans := legacySourceFactsCodeSpans(test.source)
			if got, want := facts.sourceLen(), len(test.source); got != want {
				t.Fatalf("sourceLen() = %d, want %d", got, want)
			}
			for offset, want := range legacySpans {
				if got := facts.containsCode(offset); got != want {
					t.Fatalf("containsCode(%d) = %t, want %t", offset, got, want)
				}
			}
			if facts.containsCode(-1) || facts.containsCode(len(test.source)) {
				t.Fatal("out-of-range offsets must not be code")
			}

			for start := 0; start <= len(test.source); start++ {
				for offset := start; offset <= len(test.source); offset++ {
					want := legacySourceFactsOpenBraces(test.source, start, offset, legacySpans)
					if got := facts.openBraces(start, offset); !reflect.DeepEqual(got, want) {
						t.Fatalf("openBraces(%d, %d) = %v, want %v", start, offset, got, want)
					}
				}
			}

			wantScan := legacySourceFactsSchemaScanSource(test.source)
			if got := facts.schemaScanSource(); got != wantScan {
				t.Fatalf("schemaScanSource mismatch:\n got: %q\nwant: %q", got, wantScan)
			}
			wantTokens := legacySourceFactsTokens(wantScan)
			if got := facts.schemaCandidates(); !reflect.DeepEqual(got, wantTokens) {
				t.Fatalf("schemaCandidates() = %#v, want %#v", got, wantTokens)
			}
		})
	}
}

func TestSourceFactsMatchingBracesPreserveMalformedBehavior(t *testing.T) {
	t.Parallel()

	source := `class Broken { void run() { if (true) { } } /* } */ {`
	facts := newSourceFacts(source)
	legacySpans := legacySourceFactsCodeSpans(source)
	for offset := 0; offset < len(source); offset++ {
		if source[offset] != '{' || !legacySpans[offset] {
			continue
		}
		want := legacySourceFactsMatchingBrace(source, offset, legacySpans)
		if got := facts.matchingBrace(offset); got != want {
			t.Fatalf("matchingBrace(%d) = %d, want %d", offset, got, want)
		}
	}
}

func TestSourceFactsMatchBracesBeyondInlineStackCapacity(t *testing.T) {
	t.Parallel()

	const depth = 256
	source := strings.Repeat("{", depth) + strings.Repeat("}", depth)
	facts := newSourceFacts(source)
	for openOffset := 0; openOffset < depth; openOffset++ {
		want := len(source) - openOffset - 1
		if got := facts.matchingBrace(openOffset); got != want {
			t.Fatalf("matchingBrace(%d) = %d, want %d", openOffset, got, want)
		}
	}
}

func TestSourceFactsRemainAnalysisLocal(t *testing.T) {
	t.Parallel()

	source := "class Local { void run() { } }"
	first := newSourceFacts(source)
	second := newSourceFacts(source)
	if first == second {
		t.Fatal("independent analyses must not share source facts")
	}
	if !reflect.DeepEqual(first.openBraces(0, len(source)), second.openBraces(0, len(source))) {
		t.Fatal("independent facts must remain equivalent")
	}
}

func TestSourceFactsReuseOneIndexPerAnalysisOccurrence(t *testing.T) {
	t.Parallel()

	const source = "class Local { void run() { } }"
	reads := 0
	sources := newSemaSourcesWithCaptured(nil, func(path string) (string, bool) {
		reads++
		if path != "Local.cls" {
			t.Fatalf("captured source path = %q, want Local.cls", path)
		}
		return source, true
	}, nil)
	typ := typesys.TypeSymbol{File: "Local.cls"}

	first, ok := sources.factsForType(typ)
	if !ok {
		t.Fatal("first facts lookup missed")
	}
	second, ok := sources.factsForType(typ)
	if !ok {
		t.Fatal("second facts lookup missed")
	}
	if first != second {
		t.Fatal("one analysis occurrence must reuse one immutable facts index")
	}
	if reads != 1 {
		t.Fatalf("captured source reads = %d, want 1", reads)
	}
}

func TestSourceFactsRetainSchemaCandidateBoundaries(t *testing.T) {
	t.Parallel()

	source := "Account row = new Account(Name = 'hidden'); row.Owner.Name;"
	facts := newSourceFacts(source)
	want := legacySourceFactsTokens(legacySourceFactsSchemaScanSource(source))
	if got := facts.retainedSchemaCandidateCount(); got != len(want) {
		t.Fatalf("retainedSchemaCandidateCount() = %d, want %d", got, len(want))
	}
	if got := facts.schemaCandidates(); !reflect.DeepEqual(got, want) {
		t.Fatalf("schemaCandidates() = %#v, want %#v", got, want)
	}
}

func FuzzSourceFactsBraceEquivalence(f *testing.F) {
	for _, source := range []string{
		"",
		"{}",
		"{/*}*/}",
		"{'\\'}'}",
		`{"}"}`,
		"{// }\n}",
		"class A { void f() { if (true) { } } }",
	} {
		f.Add(source, 0, len(source))
	}

	f.Fuzz(func(t *testing.T, source string, start, offset int) {
		if len(source) > 4096 {
			t.Skip()
		}
		start = sourceFactsBound(start, len(source))
		offset = sourceFactsBound(offset, len(source))
		if start > offset {
			start, offset = offset, start
		}
		legacySpans := legacySourceFactsCodeSpans(source)
		want := legacySourceFactsOpenBraces(source, start, offset, legacySpans)
		facts := newSourceFacts(source)
		if got := facts.openBraces(start, offset); !reflect.DeepEqual(got, want) {
			t.Fatalf("openBraces(%d, %d) = %v, want %v for %q", start, offset, got, want, source)
		}
		if start < len(source) {
			if got := facts.containsCode(start); got != legacySpans[start] {
				t.Fatalf("containsCode(%d) = %t, want %t for %q", start, got, legacySpans[start], source)
			}
			if source[start] == '{' && legacySpans[start] {
				wantMatch := legacySourceFactsMatchingBrace(source, start, legacySpans)
				if got := facts.matchingBrace(start); got != wantMatch {
					t.Fatalf("matchingBrace(%d) = %d, want %d for %q", start, got, wantMatch, source)
				}
			}
		}
	})
}

func FuzzSourceFactsSchemaMaskEquivalence(f *testing.F) {
	for _, source := range []string{
		"",
		"Account.Name",
		"'Account.Hidden__c'",
		"'it''s Account.Hidden__c'",
		"'escaped \\' Account.Hidden__c'",
		"// Account.Hidden__c\nContact.Name",
		"/* Account.Hidden__c */ Contact.Name",
		"/* unterminated Account.Hidden__c",
		`"// legacy schema masking treats this as a comment"`,
	} {
		f.Add(source)
	}

	f.Fuzz(func(t *testing.T, source string) {
		if len(source) > 4096 {
			t.Skip()
		}
		want := legacySourceFactsSchemaScanSource(source)
		if got := newSourceFacts(source).schemaScanSource(); got != want {
			t.Fatalf("schemaScanSource mismatch:\n got: %q\nwant: %q", got, want)
		}
	})
}

func BenchmarkSourceFactsRepeatedBraceQueries(b *testing.B) {
	source := strings.Repeat(`
public class Sample {
    void run() {
        if (true) { System.debug('{'); }
        // { ignored }
        /* } ignored */
    }
}
`, 128)
	offsets := make([]int, 0, len(source)/32)
	for offset := 0; offset < len(source); offset += 32 {
		offsets = append(offsets, offset)
	}

	b.Run("legacy", func(b *testing.B) {
		b.ReportAllocs()
		spans := legacySourceFactsCodeSpans(source)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for _, offset := range offsets {
				_ = legacySourceFactsOpenBraces(source, 0, offset, spans)
			}
		}
	})
	b.Run("indexed", func(b *testing.B) {
		b.ReportAllocs()
		facts := newSourceFacts(source)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for _, offset := range offsets {
				_ = facts.openBraces(0, offset)
			}
		}
	})
}

func BenchmarkSourceFactsSchemaScanReuse(b *testing.B) {
	source := strings.Repeat(`
public class Sample {
    String ignored = 'Account.Hidden__c';
    // Contact.Ignored__c
    Account row = new Account(Name = 'A');
}
`, 256)

	b.Run("legacy", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			for occurrence := 0; occurrence < 8; occurrence++ {
				_ = legacySourceFactsSchemaScanSource(source)
			}
		}
	})
	b.Run("indexed", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			facts := newSourceFacts(source)
			_ = facts.schemaScanSource()
		}
	})
}

func sourceFactsBound(value, limit int) int {
	if value < 0 {
		value = -value
		if value < 0 {
			value = 0
		}
	}
	if limit == 0 {
		return 0
	}
	return value % (limit + 1)
}

func legacySourceFactsCodeSpans(source string) []bool {
	spans := make([]bool, len(source))
	for i := range spans {
		spans[i] = true
	}
	for i := 0; i < len(source); i++ {
		if source[i] == '/' && i+1 < len(source) {
			switch source[i+1] {
			case '/':
				spans[i], spans[i+1] = false, false
				i += 2
				for i < len(source) && source[i] != '\n' {
					spans[i] = false
					i++
				}
				i--
				continue
			case '*':
				spans[i], spans[i+1] = false, false
				i += 2
				for i < len(source) {
					spans[i] = false
					if source[i] == '*' && i+1 < len(source) && source[i+1] == '/' {
						spans[i+1] = false
						i++
						break
					}
					i++
				}
				continue
			}
		}
		if source[i] == '\'' || source[i] == '"' {
			quote := source[i]
			spans[i] = false
			i++
			for i < len(source) {
				spans[i] = false
				if source[i] == '\\' {
					i++
					if i < len(source) {
						spans[i] = false
					}
					i++
					continue
				}
				if source[i] == quote {
					break
				}
				i++
			}
		}
	}
	return spans
}

func legacySourceFactsOpenBraces(source string, start, offset int, spans []bool) []int {
	var braces []int
	for i := start; i < offset && i < len(source); i++ {
		if i < 0 || !spans[i] {
			continue
		}
		switch source[i] {
		case '{':
			braces = append(braces, i)
		case '}':
			if len(braces) > 0 {
				braces = braces[:len(braces)-1]
			}
		}
	}
	return braces
}

func legacySourceFactsMatchingBrace(source string, start int, spans []bool) int {
	depth := 0
	for i := start; i < len(source); i++ {
		if !spans[i] {
			continue
		}
		switch source[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func legacySourceFactsSchemaScanSource(source string) string {
	out := []byte(source)
	mask := func(start, end int) {
		if start < 0 {
			start = 0
		}
		if end > len(out) {
			end = len(out)
		}
		for i := start; i < end; i++ {
			if out[i] != '\n' && out[i] != '\r' {
				out[i] = ' '
			}
		}
	}
	for i := 0; i < len(out); {
		switch {
		case i+1 < len(out) && out[i] == '/' && out[i+1] == '/':
			start := i
			i += 2
			for i < len(out) && out[i] != '\n' && out[i] != '\r' {
				i++
			}
			mask(start, i)
		case i+1 < len(out) && out[i] == '/' && out[i+1] == '*':
			start := i
			i += 2
			for i+1 < len(out) && !(out[i] == '*' && out[i+1] == '/') {
				i++
			}
			if i+1 < len(out) {
				i += 2
			}
			mask(start, i)
		case out[i] == '\'':
			start := i
			i++
			for i < len(out) {
				if out[i] == '\'' {
					if i+1 < len(out) && out[i+1] == '\'' {
						i += 2
						continue
					}
					i++
					break
				}
				if out[i] == '\\' && i+1 < len(out) {
					i += 2
					continue
				}
				i++
			}
			mask(start, i)
		default:
			i++
		}
	}
	return string(out)
}

func legacySourceFactsTokens(source string) []sourceToken {
	var tokens []sourceToken
	for i := 0; i < len(source); {
		if !sourceFactsIdentifierStart(source[i]) {
			i++
			continue
		}
		start := i
		i++
		for i < len(source) && sourceFactsIdentifierByte(source[i]) {
			i++
		}
		tokens = append(tokens, sourceToken{start: start, end: i})
	}
	return tokens
}
