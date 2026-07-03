package debuglog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/apexlog"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestAnnotateMatchesDebugSOQLAndDML(t *testing.T) {
	index := mustLoadDebugIndex(t, filepath.Join("testdata", "project"))
	log := mustReadApexLog(t, filepath.Join("testdata", "subscriber.log"))

	annotated, err := Annotate(log, index, 0)
	if err != nil {
		t.Fatal(err)
	}
	entry := findAnnotatedByDebugMessage(annotated, "start work")
	if entry == nil {
		t.Fatal("missing start work debug entry")
	}
	if entry.Best.Confidence < 0.85 {
		t.Fatalf("confidence = %.2f, want at least 0.85", entry.Best.Confidence)
	}
	if !strings.HasSuffix(entry.Best.File, "TestProcessor.cls") {
		t.Fatalf("best file = %q, want TestProcessor.cls", entry.Best.File)
	}
	if entry.Best.Line <= 0 {
		t.Fatalf("best line = %d, want positive line", entry.Best.Line)
	}
}

func TestAnnotateUsesStackFrameForException(t *testing.T) {
	index := mustLoadDebugIndex(t, filepath.Join("testdata", "project"))
	log := mustReadApexLog(t, filepath.Join("..", "apexlog", "testdata", "exception.log"))

	annotated, err := Annotate(log, index, 0)
	if err != nil {
		t.Fatal(err)
	}
	var best SourceCandidate
	found := false
	for _, entry := range annotated.Entries {
		if entry.Entry.Kind == apexlog.EntryExceptionThrown && len(entry.Candidates) > 0 {
			best = entry.Best
			found = true
			break
		}
	}
	if !found {
		t.Fatal("missing exception candidates")
	}
	if best.Confidence < 0.90 {
		t.Fatalf("best confidence = %.2f, want >= 0.90", best.Confidence)
	}
	if !strings.HasSuffix(best.File, "TestProcessor.cls") {
		t.Fatalf("best file = %q, want TestProcessor.cls", best.File)
	}
}

func TestAnnotateIgnoresSourceLineWithoutCodeUnitContext(t *testing.T) {
	index := mustLoadDebugIndex(t, filepath.Join("testdata", "project"))
	log := apexlog.Log{
		Entries: []apexlog.Entry{
			{
				Raw:       "00:00:00.000 (0)|OTHER|[3]|",
				Line:      1,
				Kind:      apexlog.EntryOther,
				Timestamp: "00:00:00.000 (0)",
				Data:      apexlog.EntryData{SourceLine: 3},
			},
		},
	}

	annotated, err := Annotate(log, index, 0)
	if err != nil {
		t.Fatal(err)
	}
	entry := annotated.Entries[0]
	if len(entry.Candidates) != 0 {
		t.Fatalf("candidates = %#v, want none", entry.Candidates)
	}
}

func TestAnnotateUsesMethodEntrySymbolBeforeLineFallback(t *testing.T) {
	root := t.TempDir()
	retrieverFile := filepath.Join(root, "ParentAffiliationRetriever.cls")
	couponFile := filepath.Join(root, "CouponManager.cls")
	if err := os.WriteFile(retrieverFile, []byte("public virtual class ParentAffiliationRetriever {\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(couponFile, []byte("public abstract class CouponManager {\n    protected CouponManager() { }\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	index := typesys.Index{Types: []typesys.TypeSymbol{
		{
			Kind:      apexast.DeclarationClass,
			Name:      "ParentAffiliationRetriever",
			Namespace: "pkg",
			File:      retrieverFile,
			Range: diagnostic.Range{
				Start: diagnostic.Position{Line: 6},
				End:   diagnostic.Position{Line: 20},
			},
		},
		{
			Kind:      apexast.DeclarationClass,
			Name:      "CouponManager",
			Namespace: "pkg",
			File:      couponFile,
			Range: diagnostic.Range{
				Start: diagnostic.Position{Line: 1},
				End:   diagnostic.Position{Line: 8},
			},
			Members: []typesys.MemberSymbol{{
				Kind: apexast.DeclarationConstructor,
				Name: "CouponManager",
				Range: diagnostic.Range{
					Start: diagnostic.Position{Line: 6},
					End:   diagnostic.Position{Line: 6},
				},
			}},
		},
	}}
	log := mustParseEditorLog(t, "00:00:00.001 (1000000)|METHOD_ENTRY|[6]|01p000000000001|pkg.ParentAffiliationRetriever.ParentAffiliationRetriever()\n")

	annotated, err := Annotate(log, index, 0)
	if err != nil {
		t.Fatal(err)
	}
	best := annotated.Entries[0].Best

	if best.File != retrieverFile {
		t.Fatalf("best = %#v, want %s", best, retrieverFile)
	}
	if best.Reason != "method symbol" {
		t.Fatalf("reason = %q, want method symbol", best.Reason)
	}
}

func TestAnnotateDoesNotMatchSourceLineWithoutFileContext(t *testing.T) {
	firstFile := filepath.Join(t.TempDir(), "First.cls")
	secondFile := filepath.Join(t.TempDir(), "Second.cls")
	index := typesys.Index{Types: []typesys.TypeSymbol{
		{
			Kind: apexast.DeclarationClass,
			Name: "First",
			File: firstFile,
			Range: diagnostic.Range{
				Start: diagnostic.Position{Line: 1},
				End:   diagnostic.Position{Line: 10},
			},
			Members: []typesys.MemberSymbol{{
				Kind: apexast.DeclarationMethod,
				Name: "run",
				Range: diagnostic.Range{
					Start: diagnostic.Position{Line: 3},
					End:   diagnostic.Position{Line: 7},
				},
			}},
		},
		{
			Kind: apexast.DeclarationClass,
			Name: "Second",
			File: secondFile,
			Range: diagnostic.Range{
				Start: diagnostic.Position{Line: 1},
				End:   diagnostic.Position{Line: 10},
			},
			Members: []typesys.MemberSymbol{{
				Kind: apexast.DeclarationMethod,
				Name: "run",
				Range: diagnostic.Range{
					Start: diagnostic.Position{Line: 3},
					End:   diagnostic.Position{Line: 7},
				},
			}},
		},
	}}
	log := apexlog.Log{Entries: []apexlog.Entry{{
		Raw:       "00:00:00.000 (0)|OTHER|[5]|",
		Line:      1,
		Kind:      apexlog.EntryOther,
		Timestamp: "00:00:00.000 (0)",
		Data:      apexlog.EntryData{SourceLine: 5},
	}}}

	annotated, err := Annotate(log, index, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := annotated.Entries[0].Candidates; len(got) != 0 {
		t.Fatalf("candidates = %#v, want none without file or code-unit context", got)
	}
}

func mustLoadDebugIndex(t *testing.T, root string) typesys.Index {
	t.Helper()
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := gladeschema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	return typesys.Build(p, s)
}

func mustReadApexLog(t *testing.T, name string) apexlog.Log {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	log, err := apexlog.Parse(strings.NewReader(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	return log
}

func findAnnotatedByDebugMessage(log AnnotatedLog, message string) *AnnotatedEntry {
	for i := range log.Entries {
		entry := &log.Entries[i]
		if entry.Entry.Kind == apexlog.EntryUserDebug && strings.Contains(entry.Entry.Data.DebugMessage, message) {
			return entry
		}
	}
	return nil
}
