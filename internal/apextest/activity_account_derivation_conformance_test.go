package apextest

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

const activityAccountFixture = "testdata/conformance/activity_account_derivation"

// activityAccountKnownGaps lists captured rows Glade does not match for a
// reason outside activity AccountId derivation.
var activityAccountKnownGaps = map[string]string{
	"U10 after What Account deleted": "deleting an Account a Draft Contract references fails locally (DELETE_FAILED); native deletes it",
}

func activityAccountNativeRows(t *testing.T) map[string]map[string][][2]string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(activityAccountFixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]map[string][][2]string{}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if lines[0] != "api\troute\tid\tobservation" {
		t.Fatalf("org.tsv header %q", lines[0])
	}
	for _, line := range lines[1:] {
		cells := strings.Split(line, "\t")
		if len(cells) != 4 {
			t.Fatalf("malformed row %q", line)
		}
		if out[cells[0]] == nil {
			out[cells[0]] = map[string][][2]string{}
		}
		out[cells[0]][cells[1]] = append(out[cells[0]][cells[1]], [2]string{cells[2], cells[3]})
	}
	return out
}

func activityAccountProject(t *testing.T, api string) string {
	t.Helper()
	root := t.TempDir()
	source := filepath.Join(activityAccountFixture, "project")
	version := regexp.MustCompile(`67\.0`)
	err := filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(source, path)
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := version.ReplaceAllString(string(raw), api)
		if filepath.Base(rel) == "LFActivityProbe2.cls" {
			// U10 is a known gap that throws locally; drop that probe block so
			// the probe returns the other rows.
			start := strings.Index(text, "        Task t10 =")
			end := strings.Index(text, "        return out;")
			if start < 0 || end < start {
				return os.ErrInvalid
			}
			text = text[:start] + text[end:]
		}
		writeFile(t, filepath.Join(root, rel), text)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// ORG-RUN.txt: permission set LFProbe was assigned before every run.
	writeCapturedAssignments(t, root, "LFProbe")
	return root
}

// TestActivityAccountDerivationNativeConformance replays the
// activity_account_derivation capture (fresh Developer scratch org, API 62.0
// and 67.0): Task and Event AccountId from WhatId (Account, Opportunity,
// Contract, custom master-detail child) or a Contact WhoId, its recompute on
// update and after a parent account change, $ObjectType in a validation rule
// and in FormulaEval, on the anonymous and isTest routes.
func TestActivityAccountDerivationNativeConformance(t *testing.T) {
	native := activityAccountNativeRows(t)
	anonymous, err := os.ReadFile(filepath.Join(activityAccountFixture, "anonymous.apex"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := activityAccountProject(t, api)
			index := loadTestIndex(t, root)
			org := capturedFixtureOrg(t, index, api)
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org, LinkProject: true})
			program, err := vm.CompileAnonymousWithOptions(string(anonymous), vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			result, runErr := runner.execute(program)
			got := map[string]string{}
			for _, line := range result.Debug {
				if _, row, ok := strings.Cut(line, "ROW|"); ok {
					id, observation, _ := strings.Cut(row, "|")
					got[id] = observation
				}
			}
			matched := 0
			for _, row := range native[api]["anonymous"] {
				if _, gap := activityAccountKnownGaps[row[0]]; gap {
					continue
				}
				if observed, ok := got[row[0]]; !ok || observed != row[1] {
					t.Errorf("anonymous %s: native <%s> glade <%s> (present=%t)", row[0], row[1], observed, ok)
					continue
				}
				matched++
			}
			if matched != 42 {
				t.Errorf("matched anonymous rows = %d; want 42 (run error: %v)", matched, runErr)
			}

			run := Run(index, Options{SelectedClasses: []string{"LFActivityProbeTest"}, NoDiskCache: true, Parallelism: 1})
			var message string
			for _, suite := range run.Suites {
				for _, c := range suite.Cases {
					if c.Problem != nil {
						message = c.Problem.Message
					}
				}
			}
			_, body, ok := strings.Cut(message, "ROWS>")
			body, _, closed := strings.Cut(body, "<ROWS")
			if !ok || !closed {
				t.Fatalf("isTest: no row message: %q", message)
			}
			gotTest := map[string]string{}
			for _, line := range strings.Split(body, "\n") {
				id, observation, _ := strings.Cut(line, "|")
				gotTest[id] = observation
			}
			for _, row := range native[api]["istest"] {
				if _, gap := activityAccountKnownGaps[row[0]]; gap {
					continue
				}
				if gotTest[row[0]] != row[1] {
					t.Errorf("isTest %s: native <%s> glade <%s>", row[0], row[1], gotTest[row[0]])
				}
			}
		})
	}
}
