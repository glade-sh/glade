package apextest

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestRequestDigestOutcomeIdentity(t *testing.T) {
	t.Cleanup(InvalidateRuntimeCaches)
	trustedClock(t)
	f := writeRequestReadFixture(t, 16)
	testSource := `@IsTest private class RequestReadsTest {
		@IsTest static void first() { System.assertEquals(0, RequestService00.value()); }
		@IsTest static void second() { System.assertEquals(1, RequestService01.value()); }
	}`
	if err := os.WriteFile(f.sources[len(f.sources)-1], []byte(testSource), 0600); err != nil {
		t.Fatal(err)
	}
	p, err := project.Load(f.root)
	if err != nil {
		t.Fatal(err)
	}
	schema, err := gladeschema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	index, artifacts := typesys.BuildWithArtifacts(p, schema)
	f.index = index
	f.options.BuildArtifacts = &artifacts
	f.options.SourceDigests = artifacts.SourceDigests
	f.options.ProjectSnapshot = &p
	f.cases = Discover(index, Options{})
	if len(f.cases) != 2 {
		t.Fatalf("discovered %d test cases, want 2", len(f.cases))
	}
	var request requestFileDigests
	var cached []string
	for _, paths := range [][]string{f.sources, f.sidecars} {
		for _, path := range paths {
			digest, err := request.digest(path)
			if err != nil {
				t.Fatal(err)
			}
			cached = append(cached, fmt.Sprintf("%s:%x", filepath.Base(path), digest))
		}
	}
	collect := func(wantOK bool) []byte {
		run := RunCasesContext(context.Background(), f.index, f.options, f.cases)
		if run.Summary().OK() != wantOK {
			t.Fatalf("unexpected request outcome: %#v", run.Summary())
		}
		run.DurationMS = 0
		for i := range run.Suites {
			run.Suites[i].DurationMS = 0
			for j := range run.Suites[i].Cases {
				run.Suites[i].Cases[j].DurationMS = 0
			}
		}
		outcome := struct {
			Run    testreport.Run `json:"run"`
			Output string         `json:"output"`
		}{run, testreportRunDetails(run)}
		data, err := json.Marshal(outcome)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	good := collect(true)
	if err := os.WriteFile(f.sources[0], []byte("public class Changed {}"), 0600); err != nil {
		t.Fatal(err)
	}
	edited := collect(false)
	result, err := json.Marshal(struct {
		Good        json.RawMessage `json:"good"`
		Edited      json.RawMessage `json:"edited"`
		Diagnostics any             `json:"diagnostics"`
		Cached      []string        `json:"cached"`
	}{good, edited, f.index.Diagnostics, cached})
	if err != nil {
		t.Fatal(err)
	}
	result = []byte(strings.ReplaceAll(string(result), filepath.ToSlash(f.root), "PROJECT"))
	want, err := os.ReadFile("testdata/request_digest_outcome.json")
	if err != nil {
		t.Fatal(err)
	}
	if string(result) != string(want) {
		t.Fatalf("request outcome differs from baseline\ngot %s\nwant %s", result, want)
	}
}
