package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// NATIVE a Customer Community or Plus user reads neither Case under the scratch default model
// testdata/conformance/external_user_case_lead_sharing/default/org.tsv:2-5 istest 62.0 67.0
// NATIVE the same with an internal Public Read/Write model
// testdata/conformance/external_user_case_lead_sharing/public_read_write/org.tsv:2-5 istest 62.0 67.0
// NATIVE the same with an internal Public Read model (Case stayed ReadWriteTransfer; Lead was Read)
// testdata/conformance/external_user_case_lead_sharing/public_read/org.tsv:2-5 istest 62.0 67.0
// NATIVE the external default was Private in all three variants
// testdata/conformance/external_user_case_lead_sharing/default/org.tsv:10-11 istest 62.0 67.0
// NATIVE a Community license refuses the Read Lead permission, so no row isolates Lead sharing
// testdata/conformance/external_user_case_lead_sharing/lead_grant/org.tsv:2-5 istest 62.0 67.0
//
// The project declares only the internal model. A Standard User control reads
// all three records (default/org.tsv:8-9).
func TestExternalUserCaseLeadSharingNativeConformance(t *testing.T) {
	const base = "testdata/conformance/external_user_case_lead_sharing"
	variants := []string{"default", "public_read_write", "public_read"}
	// Case record visibility only. The Lead reads, the Login profile's object
	// access and the Standard User control stay out: their rows differ for
	// reasons outside record sharing (portal profile object permissions and
	// UserRecordAccess rows for a named internal user).
	classes := map[string]string{"PclCcTest": "cc", "PclCcpTest": "ccp"}
	for _, variant := range variants {
		rows := readNativeTSV(t, filepath.Join(base, variant, "org.tsv"))
		for _, api := range []string{"62.0", "67.0"} {
			variant, api := variant, api
			runConformanceAPIVersion(t, variant+"/"+api, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
				copyTree(t, filepath.Join(base, "project", "force-app"), filepath.Join(root, "force-app"), api)
				if _, err := os.Stat(filepath.Join(base, variant, "objects")); err == nil {
					copyTree(t, filepath.Join(base, variant, "objects"), filepath.Join(root, "force-app/main/default/objects"), api)
				}
				copyTree(t, filepath.Join(base, variant, "project-scratch-def.json"), filepath.Join(root, "config/project-scratch-def.json"), api)
				index := loadTestIndex(t, root)
				for class, tag := range classes {
					want := ""
					for _, row := range rows {
						if row["id"] == class && row["api"] == api {
							_, want, _ = strings.Cut(row["outcome"], "Assertion Failed: ")
						}
					}
					if want == "" {
						t.Fatalf("no native row for %s %s", class, api)
					}
					run := Run(index, Options{SelectedClasses: []string{class}, SelectedMethod: "run", NoDiskCache: true, Parallelism: 1})
					if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
						t.Fatalf("%s: expected one asserted observation: %+v", class, run.Summary())
					}
					_, got, _ := strings.Cut(run.Suites[0].Cases[0].Problem.Message, "Assertion Failed: ")
					if caseVisibility(got) != caseVisibility(want) || caseVisibility(want) == "" {
						t.Errorf("%s (%s) %s/%s: Glade=%q native=%q", class, tag, variant, api, got, want)
					}
				}
			})
		}
	}
}

// caseVisibility keeps the read and edit bits of the two Case records and the
// first two query counts of an observation such as
// "cc|r=000|e=000|q=0,0,E|a=10".
func caseVisibility(observation string) string {
	parts := strings.Split(observation, "|")
	if len(parts) != 5 {
		return ""
	}
	bits := func(part, prefix string) string { return strings.TrimPrefix(part, prefix)[:2] }
	counts := strings.Split(strings.TrimPrefix(parts[3], "q="), ",")
	if len(counts) != 3 {
		return ""
	}
	return bits(parts[1], "r=") + "/" + bits(parts[2], "e=") + "/" + counts[0] + "," + counts[1]
}

func readNativeTSV(t *testing.T, path string) []map[string]string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	header := strings.Split(lines[0], "\t")
	var rows []map[string]string
	for _, line := range lines[1:] {
		cols := strings.Split(line, "\t")
		row := map[string]string{}
		for i, name := range header {
			if i < len(cols) {
				row[name] = cols[i]
			}
		}
		rows = append(rows, row)
	}
	return rows
}

func copyTree(t *testing.T, src, dst, api string) {
	t.Helper()
	info, err := os.Stat(src)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		body, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, dst, strings.ReplaceAll(string(body), "67.0", api))
		return
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		copyTree(t, filepath.Join(src, entry.Name()), filepath.Join(dst, entry.Name()), api)
	}
}
