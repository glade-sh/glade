package apextest

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// NATIVE a Customer Community or Plus user reads neither Case under the scratch default model
// testdata/conformance/external_user_case_lead_sharing/default/org.tsv:2-5 istest 62.0 67.0
// NATIVE the same with an internal Public Read/Write model
// testdata/conformance/external_user_case_lead_sharing/public_read_write/org.tsv:2-5 istest 62.0 67.0
// NATIVE the same with an internal Public Read model (Case stayed ReadWriteTransfer; Lead was Read)
// testdata/conformance/external_user_case_lead_sharing/public_read/org.tsv:2-5 istest 62.0 67.0
// NATIVE Case and Lead default to ReadWriteTransfer internally and Private externally
// testdata/conformance/external_user_case_lead_sharing/default/org.tsv:10-11 istest 62.0 67.0
// NATIVE the external defaults remain Private after internal sharing changes
// testdata/conformance/external_user_case_lead_sharing/public_read_write/org.tsv:10-11 istest 62.0 67.0
// testdata/conformance/external_user_case_lead_sharing/public_read/org.tsv:10-11 istest 62.0 67.0
// NATIVE a Community license refuses the Read Lead permission, so no row isolates Lead sharing
// testdata/conformance/external_user_case_lead_sharing/lead_grant/org.tsv:2-5 istest 62.0 67.0
//
// NATIVE a Community user reads Case and not Lead through isAccessible, a Login user too
// testdata/conformance/external_user_case_lead_sharing/default/org.tsv:2-7 istest 62.0 67.0
// NATIVE a named Standard User reads, edits and counts all three records
// testdata/conformance/external_user_case_lead_sharing/default/org.tsv:8-9 istest 62.0 67.0
//
// NATIVE explicit ReadWriteTransfer confirms the Standard User's edit bits and
// both objects' external models. The Community and Login Lead counts differ by API.
// testdata/conformance/external_user_case_lead_sharing/public_read_write_transfer/org.tsv:2-9 istest 62.0 67.0
//
// The project declares only the internal model. Each observation is compared
// whole: UserRecordAccess read and edit bits, three count queries and the Case
// and Lead isAccessible flags, plus the sharing model readback.
func TestExternalUserCaseLeadSharingNativeConformance(t *testing.T) {
	const base = "testdata/conformance/external_user_case_lead_sharing"
	const transfer = "public_read_write_transfer"
	for _, variant := range []string{"default", "public_read_write", "public_read", transfer} {
		scratchDefinition := variant
		if variant == transfer {
			scratchDefinition = "default"
		}
		rows := readNativeTSV(t, filepath.Join(base, variant, "org.tsv"))
		for _, api := range []string{"62.0", "67.0"} {
			variant, api := variant, api
			runConformanceAPIVersion(t, variant+"/"+api, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
				copyTree(t, filepath.Join(base, "project", "force-app"), filepath.Join(root, "force-app"), api)
				for _, object := range []string{"Case", "Lead"} {
					// The Public Read deployment left Case at its scratch default.
					if variant == "default" || (variant == "public_read" && object == "Case") {
						continue
					}
					path := filepath.Join("objects", object)
					copyTree(t, filepath.Join(base, variant, path), filepath.Join(root, "force-app/main/default", path), api)
				}
				copyTree(t, filepath.Join(base, scratchDefinition, "project-scratch-def.json"), filepath.Join(root, "config/project-scratch-def.json"), api)
				index := loadTestIndex(t, root)
				checked := 0
				for _, row := range rows {
					class := row["id"]
					if row["api"] != api {
						continue
					}
					checked++
					_, want, _ := strings.Cut(row["outcome"], "Assertion Failed: ")
					if want == "" {
						t.Fatalf("missing native observation for %s %s", class, api)
					}
					run := Run(index, Options{SelectedClasses: []string{class}, SelectedMethod: "run", NoDiskCache: true, Parallelism: 1})
					if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
						t.Fatalf("%s: expected one asserted observation: %+v", class, run.Summary())
					}
					_, got, _ := strings.Cut(run.Suites[0].Cases[0].Problem.Message, "Assertion Failed: ")
					if variant == "public_read" && class == "PclStdTest" {
						// This row was observed before sharing recalculated after the Lead
						// model change (lead_read_model_recalculation/org.tsv:2-5 shows the
						// later answer: no Lead edit). Only the read bits and counts are asserted.
						editBits := regexp.MustCompile(`\|e=[01]+`)
						got, want = editBits.ReplaceAllString(got, ""), editBits.ReplaceAllString(want, "")
					}
					if got != want {
						t.Errorf("%s %s/%s: Glade=%q native=%q", class, variant, api, got, want)
					}
				}
				wantRows := 5
				if variant == transfer {
					wantRows = 4
				}
				if checked != wantRows {
					t.Fatalf("checked %d native rows, want %d", checked, wantRows)
				}
			})
		}
	}
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
