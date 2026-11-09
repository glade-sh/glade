package apextest

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Fixtures whose API 67.0 rows (and, where recorded, API 62.0 rows) were
// observed natively with and without the fixture's permission set assigned.
// Each ASSIGNMENT-RUN.txt names the set to replay per API, or none.
var fixtureAssignmentRecords = []string{
	"testdata/conformance/flow_followups",
	"testdata/conformance/flow_profile_context",
	"testdata/conformance/sobject_json",
	"testdata/conformance/sobject_json2",
	"testdata/conformance/sobject_json2_namespace",
	"../../testdata/conformance/custom_address_fields",
	"../../testdata/conformance/lookup_filter_relationship_paths",
}

type fixtureAssignmentRecord struct {
	entries map[string]string
}

// fixtureTreeSHA256 digests every fixture file except the assignment record:
// sorted "<relative path>\t<file sha256>\n" lines, as ASSIGNMENT-RUN.txt states.
func fixtureTreeSHA256(t *testing.T, fixture string) string {
	t.Helper()
	var lines []string
	err := filepath.WalkDir(fixture, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, err := filepath.Rel(fixture, path)
		if err != nil || filepath.ToSlash(rel) == "ASSIGNMENT-RUN.txt" {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lines = append(lines, fmt.Sprintf("%s\t%x\n", filepath.ToSlash(rel), sha256.Sum256(body)))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(lines)
	return fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(lines, ""))))
}

// readFixtureAssignmentRecord loads and binds a native assignment record to the
// exact fixture tree and its retained rows.
func readFixtureAssignmentRecord(t *testing.T, fixture string) fixtureAssignmentRecord {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(fixture, "ASSIGNMENT-RUN.txt"))
	if err != nil {
		t.Fatal(err)
	}
	entries := map[string]string{}
	for _, line := range strings.Split(string(raw), "\n") {
		if key, value, ok := strings.Cut(line, "="); ok {
			if _, duplicate := entries[key]; duplicate {
				t.Fatalf("%s: duplicate native assignment record key %s", fixture, key)
			}
			entries[key] = value
		}
	}
	if entries["record_kind"] != "fresh_owned_capture" || entries["native_complete"] != "true" {
		t.Fatalf("%s: incomplete native assignment record", fixture)
	}
	if got := fixtureTreeSHA256(t, fixture); entries["fixture_sha256"] != got {
		t.Fatalf("%s: fixture changed since the native assignment capture", fixture)
	}
	rows, err := os.ReadFile(filepath.Join(fixture, entries["org_tsv"]))
	if err != nil || entries["org_tsv"] == "" {
		t.Fatalf("%s: native rows: %v", fixture, err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(rows)) != entries["org_tsv_sha256"] {
		t.Fatalf("%s: assignment observations are not bound to the retained rows", fixture)
	}
	return fixtureAssignmentRecord{entries: entries}
}

// recordedFixtureAssignment returns the permission set the native capture
// needed at api, with its retained metadata, or "" when the capture reproduced
// the rows unassigned. A permission set merely present in the fixture grants
// nothing; only a recorded, digest-checked assignment is replayed.
func recordedFixtureAssignment(t *testing.T, fixture, api string) (string, []byte) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(fixture, "ASSIGNMENT-RUN.txt")); os.IsNotExist(err) {
		return "", nil
	}
	record := readFixtureAssignmentRecord(t, fixture)
	access, ok := record.entries["replay_assignment:"+api]
	if !ok || record.entries["unassigned_result:"+api] == "" {
		t.Fatalf("%s: no native unassigned observation at %s", fixture, api)
	}
	if access == "none" {
		return "", nil
	}
	if record.entries["assigned_result:"+api] == "" || record.entries["permission_assignment_success:"+api] != "true" {
		t.Fatalf("%s: no native assigned observation at %s", fixture, api)
	}
	metadata, err := os.ReadFile(filepath.Join(fixture, record.entries["permission_file"]))
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(metadata)) != record.entries["permission_sha256"] {
		t.Fatalf("%s: native permission metadata changed", fixture)
	}
	if filepath.Base(record.entries["permission_file"]) != access+".permissionset-meta.xml" {
		t.Fatalf("%s: recorded assignment %s does not name the retained permission set", fixture, access)
	}
	return access, metadata
}

// writeRecordedFixtureAssignment configures a copied project with the recorded
// assignment for api. It returns the replayed set, or "" when none is recorded.
func writeRecordedFixtureAssignment(t *testing.T, fixture, api, projectRoot, permissionDir string) string {
	t.Helper()
	access, metadata := recordedFixtureAssignment(t, fixture, api)
	if access == "" {
		return ""
	}
	writeFile(t, filepath.Join(projectRoot, permissionDir, access+".permissionset-meta.xml"), string(metadata))
	writeFile(t, filepath.Join(projectRoot, "glade.yml"), "test:\n  permissionSets: ["+access+"]\n")
	return access
}

// Each record carries with/without observations at both captured APIs and
// replays a set only where the unassigned rows failed natively.
func TestFixtureAssignmentRecords(t *testing.T) {
	for _, fixture := range fixtureAssignmentRecords {
		t.Run(filepath.Base(fixture), func(t *testing.T) {
			record := readFixtureAssignmentRecord(t, fixture)
			if record.entries["source_apis"] != "62.0,67.0" {
				t.Fatalf("captured APIs = %q", record.entries["source_apis"])
			}
			for _, api := range []string{"62.0", "67.0"} {
				if record.entries["deployment_success:"+api] != "true" || record.entries["unassigned_result:"+api] == "" {
					t.Fatalf("missing native unassigned observation at %s", api)
				}
				access := record.entries["replay_assignment:"+api]
				if access == "" {
					t.Fatalf("no replay decision at %s", api)
				}
				if access != "none" {
					if name, _ := recordedFixtureAssignment(t, fixture, api); name != access {
						t.Fatalf("replay at %s = %q, record names %q", api, name, access)
					}
				}
			}
			if record.entries["replay_assignment:67.0"] == "none" || record.entries["assigned_result:67.0"] == "" {
				t.Fatal("API 67.0 assignment is not backed by an assigned observation")
			}
		})
	}
}
