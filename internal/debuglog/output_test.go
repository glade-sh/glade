package debuglog

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexlog"
)

func TestWriteTextIncludesHighConfidenceAnnotation(t *testing.T) {
	index := mustLoadDebugIndex(t, filepath.Join("testdata", "project"))
	log := mustReadApexLog(t, filepath.Join("testdata", "subscriber.log"))
	annotated, err := Annotate(log, index, 0)
	if err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := WriteText(&out, annotated, 0.5); err != nil {
		t.Fatal(err)
	}

	contents := out.String()
	if !strings.Contains(contents, "|USER_DEBUG|[3]|INFO|start work") {
		t.Fatalf("missing raw debug line: %s", contents)
	}
	if !strings.Contains(contents, "confidence=0.90") {
		t.Fatalf("missing confidence marker: %s", contents)
	}
	if !strings.Contains(contents, "TestProcessor.cls") {
		t.Fatalf("missing source annotation: %s", contents)
	}
}

func TestWriteTextSuppressesLowConfidenceAnnotation(t *testing.T) {
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

	var out bytes.Buffer
	if err := WriteText(&out, annotated, 0.75); err != nil {
		t.Fatal(err)
	}
	contents := out.String()
	if strings.Contains(contents, "=>") {
		t.Fatalf("did not expect weak annotation: %s", contents)
	}
}

func TestWriteJSONIncludesCandidates(t *testing.T) {
	index := mustLoadDebugIndex(t, filepath.Join("testdata", "project"))
	log := mustReadApexLog(t, filepath.Join("testdata", "subscriber.log"))
	annotated, err := Annotate(log, index, 0)
	if err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := WriteJSON(&out, annotated); err != nil {
		t.Fatal(err)
	}
	contents := out.String()
	for _, want := range []string{`"entries"`, `"candidates"`, `"best"`} {
		if !strings.Contains(contents, want) {
			t.Fatalf("missing %s in output: %s", want, contents)
		}
	}
}
