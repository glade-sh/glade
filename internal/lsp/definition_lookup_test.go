package lsp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// definitionRange builds a symbol range that toLSPRange maps to the given
// LSP line and character.
func definitionRange(line, character int) diagnostic.Range {
	start := diagnostic.Position{Line: line + 1, Column: character + 1}
	return diagnostic.Range{Start: start, End: diagnostic.Position{Line: start.Line, Column: start.Column + 6}}
}

// definitionLookupProject returns an in-memory project with uniquely named
// classes and members, one trigger per fifty classes, and objects with
// custom fields. No file has to exist on disk.
func definitionLookupProject(classes, members int) typesys.Index {
	var idx typesys.Index
	for c := 0; c < classes; c++ {
		typ := typesys.TypeSymbol{
			Kind:  apexast.DeclarationClass,
			Name:  fmt.Sprintf("LookupClass%04d", c),
			File:  fmt.Sprintf("LookupClass%04d.cls", c),
			Range: definitionRange(10, c+1),
		}
		for m := 0; m < members; m++ {
			typ.Members = append(typ.Members, typesys.MemberSymbol{
				Kind:  apexast.DeclarationMethod,
				Name:  fmt.Sprintf("member%dof%04d", m, c),
				Range: definitionRange(20+m, c+1),
			})
		}
		idx.Types = append(idx.Types, typ)
	}
	for t := 0; t*50 < classes; t++ {
		idx.Triggers = append(idx.Triggers, typesys.TriggerSymbol{
			Name:       fmt.Sprintf("LookupTrigger%02d", t),
			ObjectName: "Account",
			File:       fmt.Sprintf("LookupTrigger%02d.trigger", t),
			Range:      definitionRange(60+t, 2),
		})
	}
	for o := 0; o < 3; o++ {
		obj := schema.Object{Name: fmt.Sprintf("LookupObject%02d", o)}
		for f := 0; f < 40; f++ {
			obj.Fields = append(obj.Fields, schema.Field{Name: fmt.Sprintf("field%02dof%02d__c", f, o), Type: "Text"})
		}
		idx.Objects = append(idx.Objects, obj)
	}
	return idx
}

// linearDefinitionForName is the symbol scan definitionForName ran before the
// per-generation name index: every type and its members, then triggers, then
// objects and their fields, first match wins. The name index must keep
// agreeing with this scan, including which match wins when names collide.
func linearDefinitionForName(h *requestHandler, name string) (Location, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return Location{}, false
	}
	for _, typ := range h.index.Types {
		if strings.EqualFold(typ.Name, name) {
			return Location{URI: uriFromPath(typ.File), Range: toLSPRange(typ.Range)}, true
		}
		for _, member := range typ.Members {
			if strings.EqualFold(member.Name, name) || strings.EqualFold(typ.Name+"."+member.Name, name) {
				return Location{URI: uriFromPath(typ.File), Range: toLSPRange(member.Range)}, true
			}
		}
	}
	for _, trigger := range h.index.Triggers {
		if strings.EqualFold(trigger.Name, name) {
			return Location{URI: uriFromPath(trigger.File), Range: toLSPRange(trigger.Range)}, true
		}
	}
	for _, object := range h.index.Objects {
		if strings.EqualFold(object.Name, name) {
			return Location{URI: schemaURI(object.Name), Range: Range{}}, true
		}
		for _, field := range object.Fields {
			if strings.EqualFold(field.Name, name) || strings.EqualFold(object.Name+"."+field.Name, name) {
				return Location{URI: schemaURI(object.Name, "fields", field.Name), Range: Range{}}, true
			}
		}
	}
	return Location{}, false
}

func TestDefinitionLookupMatchesLinearScan(t *testing.T) {
	idx := definitionLookupProject(40, 3)
	// Fold probes. EqualFold matches names whose ToLower differs (U+017F long
	// s shares the s fold cycle) and rejects pairs that only full folding
	// joins, and empty owner names still form qualified member names.
	idx.Types = append(idx.Types,
		typesys.TypeSymbol{Kind: apexast.DeclarationClass, Name: "ſize", File: "LongS.cls", Range: definitionRange(70, 4)},
		typesys.TypeSymbol{Kind: apexast.DeclarationClass, Name: "Straße", File: "Eszett.cls", Range: definitionRange(71, 4)},
		typesys.TypeSymbol{
			Kind:    apexast.DeclarationClass,
			Name:    "",
			File:    "Unnamed.cls",
			Members: []typesys.MemberSymbol{{Kind: apexast.DeclarationMethod, Name: "orphan", Range: definitionRange(72, 4)}},
		},
	)
	idx.Objects = append(idx.Objects, schema.Object{Name: "", Fields: []schema.Field{{Name: "orphan", Type: "Text"}}})
	names := []string{
		"", "   ", " LookupClass0007 ", "LookupClass", "nosuchname", "member1of0007.",
		"SIZE", "ſize", "ſIZE", "STRASSE", "straße", "İ",
		"orphan", ".orphan", "ORPHAN",
		"LookupTrigger00", "lookuptrigger00",
		"LookupObject01", "lookupobject01",
		"lookupobject02.field39of02__c", "LookupObject02.Field39Of02__C",
		"LookupClass0007.member1of0007", "LOOKUPCLASS0007.MEMBER1OF0007", "LookupClass0007.Member1Of0007",
		"LookupClass0007.member1of0008",
	}
	for c := 0; c < 40; c++ {
		names = append(names, fmt.Sprintf("LookupClass%04d", c), strings.ToLower(fmt.Sprintf("LookupClass%04d", c)))
	}
	for m := 0; m < 3; m++ {
		c := (m * 13) % 40
		names = append(names, fmt.Sprintf("Member%dOf%04d", m, c), fmt.Sprintf("lookupclass%04d.member%dof%04d", c, m, c))
	}
	for f := 0; f < 40; f += 7 {
		names = append(names, fmt.Sprintf("Field%02dOf00__c", f))
	}
	h := NewHandler(idx)
	snap := h.snapshot()
	for _, name := range names {
		gotLoc, gotOK := snap.definitionForName(name)
		wantLoc, wantOK := linearDefinitionForName(snap, name)
		if gotOK != wantOK || gotLoc != wantLoc {
			t.Fatalf("definitionForName(%q) = (%v, %v), linear scan = (%v, %v)", name, gotLoc, gotOK, wantLoc, wantOK)
		}
	}
}

func TestDefinitionLookupFirstMatchKeepsScanOrder(t *testing.T) {
	cases := []struct {
		query string
		index typesys.Index
		want  Location
	}{
		{
			// A type name beats a later type's member.
			query: "shared",
			index: typesys.Index{Types: []typesys.TypeSymbol{
				{Name: "Shared", File: "typea.cls", Range: definitionRange(11, 3)},
				{Name: "Later", File: "typel.cls", Members: []typesys.MemberSymbol{{Name: "shared", Range: definitionRange(12, 3)}}},
			}},
			want: Location{URI: uriFromPath("typea.cls"), Range: toLSPRange(definitionRange(11, 3))},
		},
		{
			// A type name beats its own member with the same name.
			query: "shared",
			index: typesys.Index{Types: []typesys.TypeSymbol{
				{Name: "Shared", File: "typea.cls", Range: definitionRange(11, 3), Members: []typesys.MemberSymbol{{Name: "shared", Range: definitionRange(12, 3)}}},
			}},
			want: Location{URI: uriFromPath("typea.cls"), Range: toLSPRange(definitionRange(11, 3))},
		},
		{
			// A member beats an object field for the unqualified and the
			// qualified spelling: types are scanned before objects.
			query: "shared",
			index: typesys.Index{
				Types:   []typesys.TypeSymbol{{Name: "Beta", File: "typeb.cls", Members: []typesys.MemberSymbol{{Name: "shared", Range: definitionRange(13, 3)}}}},
				Objects: []schema.Object{{Name: "Beta", Fields: []schema.Field{{Name: "shared", Type: "Text"}}}},
			},
			want: Location{URI: uriFromPath("typeb.cls"), Range: toLSPRange(definitionRange(13, 3))},
		},
		{
			query: "beta.shared",
			index: typesys.Index{
				Types:   []typesys.TypeSymbol{{Name: "Beta", File: "typeb.cls", Members: []typesys.MemberSymbol{{Name: "shared", Range: definitionRange(13, 3)}}}},
				Objects: []schema.Object{{Name: "Beta", Fields: []schema.Field{{Name: "shared", Type: "Text"}}}},
			},
			want: Location{URI: uriFromPath("typeb.cls"), Range: toLSPRange(definitionRange(13, 3))},
		},
		{
			// A trigger beats an object of the same name.
			query: "shared",
			index: typesys.Index{
				Triggers: []typesys.TriggerSymbol{{Name: "Shared", File: "shared.trigger", Range: definitionRange(14, 3)}},
				Objects:  []schema.Object{{Name: "shared", Fields: []schema.Field{{Name: "id", Type: "Text"}}}},
			},
			want: Location{URI: uriFromPath("shared.trigger"), Range: toLSPRange(definitionRange(14, 3))},
		},
		{
			// An earlier object's field beats a later object's name: the scan
			// interleaves each object's name with its own fields.
			query: "shared",
			index: typesys.Index{Objects: []schema.Object{
				{Name: "First", Fields: []schema.Field{{Name: "shared", Type: "Text"}}},
				{Name: "Shared", Fields: []schema.Field{{Name: "id", Type: "Text"}}},
			}},
			want: Location{URI: schemaURI("First", "fields", "shared"), Range: Range{}},
		},
		{
			// An object's own name is checked before its fields.
			query: "shared",
			index: typesys.Index{Objects: []schema.Object{
				{Name: "Shared", Fields: []schema.Field{{Name: "shared", Type: "Text"}}},
			}},
			want: Location{URI: schemaURI("Shared"), Range: Range{}},
		},
		{
			// An earlier type's member beats a later type's member.
			query: "shared",
			index: typesys.Index{Types: []typesys.TypeSymbol{
				{Name: "Early", File: "early.cls", Members: []typesys.MemberSymbol{{Name: "shared", Range: definitionRange(15, 3)}}},
				{Name: "Late", File: "late.cls", Members: []typesys.MemberSymbol{{Name: "shared", Range: definitionRange(16, 3)}}},
			}},
			want: Location{URI: uriFromPath("early.cls"), Range: toLSPRange(definitionRange(15, 3))},
		},
		{
			// Two types with the same name: the earlier one owns the
			// qualified member name.
			query: "beta.shared",
			index: typesys.Index{Types: []typesys.TypeSymbol{
				{Name: "Beta", File: "beta1.cls", Members: []typesys.MemberSymbol{{Name: "shared", Range: definitionRange(17, 3)}}},
				{Name: "Beta", File: "beta2.cls", Members: []typesys.MemberSymbol{{Name: "shared", Range: definitionRange(18, 3)}}},
			}},
			want: Location{URI: uriFromPath("beta1.cls"), Range: toLSPRange(definitionRange(17, 3))},
		},
	}
	for _, tc := range cases {
		snap := NewHandler(tc.index).snapshot()
		got, ok := snap.definitionForName(tc.query)
		if !ok || got != tc.want {
			t.Fatalf("definitionForName(%q) = (%v, %v), want (%v, true)", tc.query, got, ok, tc.want)
		}
	}
}

// TestDefinitionLookupAfterEdit covers the name index invalidation: an edit
// that renames a class installs a new generation, and the next request must
// resolve the edited names, not the ones the old index cached.
func TestDefinitionLookupAfterEdit(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "classes", "Draft.cls")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sfdx-project.json"), []byte(`{"packageDirectories":[{"path":"classes","default":true}],"sourceApiVersion":"67.0"}`), 0644); err != nil {
		t.Fatal(err)
	}
	initial := "public class Draft { public void run() { Integer x = 1; } }"
	if err := os.WriteFile(path, []byte(initial), 0644); err != nil {
		t.Fatal(err)
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(typesys.Build(p, schema.Schema{}))
	defer h.parser.Close()
	uri := uriFromPath(path)
	h.DidOpen(DidOpenTextDocumentParams{TextDocument: TextDocumentItem{URI: uri, Version: 1, Text: initial}})
	opened := h.snapshot()
	if _, ok := opened.definitionForName("Draft"); !ok {
		t.Fatal("class name did not resolve before the edit")
	}
	if _, ok := opened.definitionForName("run"); !ok {
		t.Fatal("member name did not resolve before the edit")
	}
	edited := "public class Renamed { public void renamed() { Integer x = 1; } }"
	if _, err := h.DidChange(DidChangeTextDocumentParams{
		TextDocument:   VersionedTextDocumentIdentifier{URI: uri, Version: 2},
		ContentChanges: []TextDocumentContentChangeEvent{{Text: edited}},
	}); err != nil {
		t.Fatal(err)
	}
	next := h.snapshot()
	if _, ok := next.definitionForName("Renamed"); !ok {
		t.Fatal("edited class name did not resolve after the edit")
	}
	if _, ok := next.definitionForName("renamed"); !ok {
		t.Fatal("edited member name did not resolve after the edit")
	}
	if _, ok := next.definitionForName("Draft"); ok {
		t.Fatal("removed class name still resolves after the edit")
	}
	if _, ok := next.definitionForName("run"); ok {
		t.Fatal("removed member name still resolves after the edit")
	}
	// The request pinned to the old generation keeps that generation's answer.
	if _, ok := opened.definitionForName("Draft"); !ok {
		t.Fatal("the old generation lost its own class name")
	}
}

// A linear scan rebuilds the declaration URI on every hit, so two lookups
// allocate eight times on the scan; an index hit returns the location the
// generation stored, and only the query normalization allocates. The budget
// is the load-insensitive assertion that lookups stop walking the symbol
// table and use the per-generation index.
func TestDefinitionLookupReuseBudget(t *testing.T) {
	h := NewHandler(definitionLookupProject(1200, 8))
	snap := h.snapshot()
	if _, ok := snap.definitionForName("member7of1199"); !ok {
		t.Fatal("expected the indexed member to resolve")
	}
	if _, ok := snap.definitionForName("nosuchname"); ok {
		t.Fatal("unexpected resolution")
	}
	allocs := testing.AllocsPerRun(32, func() {
		if _, ok := snap.definitionForName("Member7Of1199"); !ok {
			t.Error("indexed member stopped resolving")
		}
		if _, ok := snap.definitionForName("NoSuchName"); ok {
			t.Error("unexpected resolution")
		}
	})
	if allocs > 4 {
		t.Fatalf("%v allocations for two name lookups; the symbol table is rescanned", allocs)
	}
}

// BenchmarkDefinitionForName measures name resolution on a large synthetic
// project: the hit resolves a member of the last class, the miss walks past
// every symbol.
func BenchmarkDefinitionForName(b *testing.B) {
	h := NewHandler(definitionLookupProject(2500, 8))
	b.Run("hit", func(b *testing.B) {
		snap := h.snapshot()
		if _, ok := snap.definitionForName("member7of2499"); !ok {
			b.Fatal("missing benchmark symbol")
		}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, ok := snap.definitionForName("member7of2499"); !ok {
				b.Fatal("missing benchmark symbol")
			}
		}
	})
	b.Run("miss", func(b *testing.B) {
		snap := h.snapshot()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, ok := snap.definitionForName("nosuchsymbol"); ok {
				b.Fatal("unexpected symbol")
			}
		}
	})
}
