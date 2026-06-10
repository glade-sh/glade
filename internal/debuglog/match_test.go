package debuglog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexlog"
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

func TestAnnotateKeepsWeakMatchesWeak(t *testing.T) {
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
	weak := annotated.Entries[0]
	if weak.Best.Confidence > 0.50 {
		t.Fatalf("weak confidence = %.2f, want <= 0.50", weak.Best.Confidence)
	}
	if weak.Best.Line <= 0 {
		t.Fatalf("weak line = %d, want positive line", weak.Best.Line)
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
