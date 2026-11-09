package sema

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// referenceDirectTypeFieldBindings is the scan semaDirectTypeFieldBindings
// performed before nested block interiors were removed from its input: every
// declaration match in the whole type body, filtered to type scope.
func referenceDirectTypeFieldBindings(source string, rangeStart, rangeEnd int, spans semaCodeSpans) []semaScopedBinding {
	var fields []semaScopedBinding
	for _, match := range semaBindingDeclarationMatches(source[rangeStart+1 : rangeEnd]) {
		if !semaDeclarationAtTypeScope(source, rangeStart+1, rangeStart+1+match[4], spans) {
			continue
		}
		fields = append(fields, semaScopedBinding{name: source[rangeStart+1+match[4] : rangeStart+1+match[5]], typeName: strings.TrimSpace(source[rangeStart+1+match[2] : rangeStart+1+match[3]]), field: true, static: semaBindingStaticHeader(source[rangeStart+1+match[0] : rangeStart+1+match[2]])})
	}
	return fields
}

// typeScopeFieldBindingSources are shapes the compaction must handle: property
// bodies, collection initializers with commas, multi-declarator fields, nested
// types, parameters, blocks that never close, unbalanced interiors, and
// annotation arguments that reach a brace.
var typeScopeFieldBindingSources = []string{
	"public with sharing class A {\n\tpublic String Name { get; set; }\n\tprivate static Integer count = 0;\n\tList<Integer> seeds = new List<Integer>{1, 2, 3}, more;\n\tpublic void run(Account acct, Map<String, List<Contact>> byId) {\n\t\tInteger local = 1;\n\t\tfor (Contact c : [SELECT Id FROM Contact WHERE AccountId = :acct.Id]) { local++; }\n\t}\n\tpublic class Inner {\n\t\tDecimal amount;\n\t\tpublic Inner(Decimal a) { amount = a; }\n\t}\n\tfinal String tail = 'brace { in string }';\n}\n",
	"public class B {\n\tInteger a, b, c = 2;\n\tstatic final Map<String, Set<Id>> index = new Map<String, Set<Id>>{'k' => new Set<Id>()}, other = null;\n\t@TestVisible private Boolean flag;\n\t@InvocableMethod(label='Run' description='desc, with comma')\n\tpublic static List<String> run(List<String> input) { return input; }\n\tDate d1 = Date.today(), d2;\n}\n",
	"public class C {\n\tInteger before;\n\tvoid m() {\n\t\tString s = ')';\n\t\tInteger x = a[0;\n\t}\n\tInteger after, sibling;\n}\n",
	"public class D {\n\tInteger before;\n\tvoid m() {\n\t\tif (x) {\n\t\tInteger unterminated;\n\tInteger after;\n",
	"public class E {\n\t@Foo(x={1}) Integer odd, pair;\n\tvoid m() { Integer inside; }\n\tInteger last;\n}\n",
	"trigger T on Account (before insert) {\n\tInteger n = Trigger.new.size();\n\tfor (Account a : Trigger.new) { Integer inner = 1; }\n}\n",
	"public class F {\n\tvoid m() { Integer x = 1; } Integer rightAfter, another;\n\tInteger g() { return 1; }\n\tList<Account> accounts = [SELECT Id FROM Account], copy;\n}\n",
	"public class G {\n\tpublic String Name {\n\t\tget { return name; }\n\t\tset { name = value; }\n\t}\n\tString name;\n\tpublic G(String n, Integer k) { name = n; }\n\tstatic { Integer staticBlock = 1; }\n\tMap<String, Map<String, List<Integer>>> deep = new Map<String, Map<String, List<Integer>>>(), deepTwin;\n}\n",
	"public class H {\n\tInteger a;\n\tvoid m() { ) }\n\tInteger b, c;\n}\n",
	"public class I {\n\tInteger a;\n\tvoid m() { ( }\n\tInteger b, c;\n}\n",
	"public class J {\n\tInteger a;\n\tvoid m() { List<Integer> q = new List<Integer> { 1 }; }\n\tInteger b, c;\n}\n",
}

func typeScopeBodiesOf(source string) ([][2]int, semaCodeSpans) {
	spans := newSemaCodeSpans(source)
	var ranges [][2]int
	for _, brace := range spans.facts.braces {
		if !brace.open {
			continue
		}
		headerStart := semaHeaderStart(source, brace.offset, spans)
		if !semaTypeHeader.MatchString(source[headerStart:brace.offset]) {
			continue
		}
		end := spans.matchingBrace(brace.offset)
		if end < 0 {
			end = len(source)
		}
		ranges = append(ranges, [2]int{brace.offset, end})
	}
	return ranges, spans
}

func assertTypeScopeFieldBindingsEquivalent(t *testing.T, name, source string) (types, fallbacks int) {
	t.Helper()
	resolver := newSemaBindingResolver(source, newSemaCodeSpans(source))
	ranges, spans := typeScopeBodiesOf(source)
	for _, typeRange := range ranges {
		want := referenceDirectTypeFieldBindings(resolver.declarations, typeRange[0], typeRange[1], spans)
		got := semaDirectTypeFieldBindings(resolver.declarations, typeRange[0], typeRange[1], spans)
		if !reflect.DeepEqual(want, got) {
			t.Fatalf("%s: type body at %d-%d: field bindings differ\nwant %+v\ngot  %+v", name, typeRange[0], typeRange[1], want, got)
		}
		types++
		if blocks := spans.nestedBlocks(typeRange[0]+1, typeRange[1]); len(blocks) > 0 {
			if _, cuts := semaTypeScopeDeclarationText(resolver.declarations[typeRange[0]+1:typeRange[1]], blocks, typeRange[0]+1); cuts == nil {
				fallbacks++
			}
		}
	}
	return types, fallbacks
}

func TestTypeScopeFieldBindingsMatchFullScan(t *testing.T) {
	for i, source := range typeScopeFieldBindingSources {
		assertTypeScopeFieldBindingsEquivalent(t, "sample "+string(rune('A'+i)), source)
	}
}

// TestTypeScopeFieldBindingsMatchFullScanOnFixtures compares the compacted scan
// with the full scan on every Apex source under the repository. Set
// GLADE_SEMA_EQUIV_DIRS (os.PathListSeparator-separated) to add directories.
func TestTypeScopeFieldBindingsMatchFullScanOnFixtures(t *testing.T) {
	roots := []string{filepath.Join("..", "..")}
	if extra := os.Getenv("GLADE_SEMA_EQUIV_DIRS"); extra != "" {
		roots = append(roots, strings.Split(extra, string(os.PathListSeparator))...)
	}
	files, types, fallbacks := 0, 0, 0
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if entry.IsDir() {
				if base := entry.Name(); base == "node_modules" || base == ".git" || base == "web" {
					return filepath.SkipDir
				}
				return nil
			}
			if ext := filepath.Ext(path); ext != ".cls" && ext != ".trigger" {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			files++
			compared, kept := assertTypeScopeFieldBindingsEquivalent(t, path, string(data))
			types += compared
			fallbacks += kept
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if files == 0 || types == 0 {
		t.Fatalf("no Apex sources compared (files=%d types=%d)", files, types)
	}
	t.Logf("compared %d type bodies in %d files; %d bodies kept their full text", types, files, fallbacks)
}

func TestTypeScopeDeclarationTextCompactsBalancedBlocks(t *testing.T) {
	source := "class A {\n\tInteger a;\n\tvoid m() {\n\t\tInteger x = f(1, 2);\n\t}\n\tInteger b;\n}\n"
	ranges, spans := typeScopeBodiesOf(source)
	if len(ranges) != 1 {
		t.Fatalf("type ranges = %v", ranges)
	}
	start, end := ranges[0][0]+1, ranges[0][1]
	body := source[start:end]
	text, cuts := semaTypeScopeDeclarationText(body, spans.nestedBlocks(start, end), start)
	want := "\n\tInteger a;\n\tvoid m() {}\n\tInteger b;\n"
	if text != want {
		t.Fatalf("compacted text = %q, want %q", text, want)
	}
	if len(cuts) != 1 {
		t.Fatalf("cuts = %+v", cuts)
	}
	for compacted := range text {
		original := cuts.offset(compacted)
		if text[compacted] != body[original] {
			t.Fatalf("offset %d maps to %d: %q vs %q", compacted, original, text[compacted], body[original])
		}
	}
	if got := cuts.offset(len(text)); got != len(body) {
		t.Fatalf("end offset maps to %d, want %d", got, len(body))
	}
}

func TestTypeScopeDeclarationTextKeepsUnsafeBodies(t *testing.T) {
	for _, source := range []string{
		"class A {\n\tvoid m() { ) }\n\tInteger b;\n}\n",
		"class A {\n\tvoid m() { ( }\n\tInteger b;\n}\n",
		"class A {\n\t@Foo(x={1}) Integer a;\n\tvoid m() { }\n}\n",
	} {
		ranges, spans := typeScopeBodiesOf(source)
		start, end := ranges[0][0]+1, ranges[0][1]
		body := source[start:end]
		text, cuts := semaTypeScopeDeclarationText(body, spans.nestedBlocks(start, end), start)
		if text != body || cuts != nil {
			t.Fatalf("%q: compacted to %q with cuts %+v, want the body unchanged", source, text, cuts)
		}
	}
}

func TestSourceFactsNestedBlocks(t *testing.T) {
	source := "{ a { b } c { d { e } } } f } g {"
	facts := newSourceFacts(source)
	got := facts.nestedBlocks(1, len(source))
	want := [][2]int{{4, 8}, {12, 22}, {len(source) - 1, len(source)}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("nestedBlocks = %v, want %v", got, want)
	}
	if got := facts.nestedBlocks(0, len(source)); !reflect.DeepEqual(got, [][2]int{{0, 24}, {len(source) - 1, len(source)}}) {
		t.Fatalf("nestedBlocks from 0 = %v", got)
	}
}
