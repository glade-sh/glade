package sema

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Native rows: unknown_sobject_field/org.tsv. Each row is one statement block
// compiled natively as anonymous Apex and as a named class method at 62.0 and
// 67.0. A field the object does not have is a compile error on both routes:
// "Variable does not exist: <field>" for a read, assignment, compound
// assignment, increment or relationship path, and "Field does not exist:
// <field> on <Object>" for a constructor initializer. String put/get stay
// runtime-only. Case variants of real fields compile.

const unknownSObjectFieldDir = "../apextest/testdata/conformance/unknown_sobject_field/"

type unknownFieldRow struct {
	api, org, route, id string
	compiled            bool
	problem             string
	problemCount        int
}

// unknownFieldParkedReason names the rows whose native answer Glade cannot
// decide from its own metadata. Each stays at origin/main behavior (open); the
// reason is the gap. A parked row is asserted in neither direction.
func unknownFieldParkedReason(row unknownFieldRow) string {
	id := row.id
	switch {
	case strings.HasPrefix(id, "N_") && !strings.Contains(id, "_put") && !strings.HasPrefix(id, "N_wid_"),
		strings.HasPrefix(id, "NS_") && (strings.Contains(id, "_foreign_") || strings.Contains(id, "_pkgapp_")):
		return "a name in another namespace stays open: an installed package supplies it and the project may not declare it"
	case strings.Contains(id, "_c_ctor") || strings.Contains(id, "_unk_own_ctor") || strings.Contains(id, "_unk_u_ctor"):
		return "an SObject literal infers its own named fields onto the object (schema_inference.go)"
	case id == "K_case_Language_read" || id == "K_case_SourceId_read":
		return "feature-gated Case fields: absent from the plain org, present in the captured portal and social configurations (case_user_surface_gated), so they stay accepted"
	case strings.HasSuffix(id, "_case_SourceId_read") || strings.HasSuffix(id, "_case_SourceId_lower") || strings.HasSuffix(id, "_case_SourceId_upper"):
		return "Case.SourceId visibility follows the case_source_evidence rows, not the field list"
	case id == "R13_case_owner_unk_read":
		return "fields of the polymorphic Owner parent (User or Group) have no single closed surface"
	case !row.compiled && (strings.HasSuffix(row.problem, "__c") || strings.HasSuffix(row.problem, "__r")) &&
		!strings.Contains(id, "wid") && !strings.HasPrefix(id, "F_"):
		return "a custom field name on a standard object stays open: project field files are not the org's complete list unless a snapshot asserts it (standard_field_access)"
	case strings.HasSuffix(id, "_RecordTypeId_read"):
		return "RecordTypeId follows the declared record-type shape (record_type_shape rows), not the field list"
	}
	return ""
}

func readUnknownFieldRows(t *testing.T) ([]unknownFieldRow, map[string]string) {
	t.Helper()
	data, err := os.ReadFile(unknownSObjectFieldDir + "cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var bodies map[string]string
	if err := json.Unmarshal(data, &bodies); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(unknownSObjectFieldDir + "org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var rows []unknownFieldRow
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	scanner.Scan()
	for scanner.Scan() {
		cols := strings.SplitN(scanner.Text(), "\t", 5)
		if len(cols) != 5 {
			t.Fatalf("bad row %q", scanner.Text())
		}
		var obs struct {
			Compiled       bool    `json:"compiled"`
			CompileProblem *string `json:"compileProblem"`
			ProblemCount   int     `json:"problemCount"`
		}
		if err := json.Unmarshal([]byte(cols[4]), &obs); err != nil {
			t.Fatal(err)
		}
		row := unknownFieldRow{api: cols[0], org: cols[1], route: cols[2], id: cols[3], compiled: obs.Compiled, problemCount: obs.ProblemCount}
		if obs.CompileProblem != nil {
			row.problem = *obs.CompileProblem
		}
		rows = append(rows, row)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(rows) < 1000 {
		t.Fatalf("native rows: %d", len(rows))
	}
	return rows, bodies
}

// unknownFieldIndex copies a testdata project, adds the extra files (paths
// relative to the project root) and builds the index.
func unknownFieldIndex(t *testing.T, projectDir string, extra map[string]string) typesys.Index {
	t.Helper()
	root := t.TempDir()
	err := filepath.Walk(unknownSObjectFieldDir+projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(unknownSObjectFieldDir+projectDir, path)
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		dest := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		return os.WriteFile(dest, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	for name, contents := range extra {
		dest := filepath.Join(root, "force-app", "main", "default", filepath.FromSlash(name))
		if strings.HasPrefix(name, "/") {
			dest = filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(name, "/")))
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			t.Fatal(err)
		}
		writeSemaFile(t, dest, contents)
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	index := typesys.Build(p, s)
	if index.HasErrors() {
		t.Fatalf("index errors: %v", index.Diagnostics)
	}
	return index
}

// unknownFieldSources renders one row's body as the source its route compiles.
func unknownFieldSources(row unknownFieldRow, body string) map[string]string {
	meta := func(kind string) string {
		return `<` + kind + ` xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>` + row.api + `</apiVersion><status>Active</status></` + kind + `>`
	}
	name := "K_" + row.id
	switch row.route {
	case "trigger":
		object := "Account"
		if strings.HasPrefix(row.id, "T_wid") {
			object = "Widget__c"
		}
		return map[string]string{
			"triggers/" + row.id + ".trigger":          "trigger " + row.id + " on " + object + " (before insert) {\n  " + body + "\n}\n",
			"triggers/" + row.id + ".trigger-meta.xml": meta("ApexTrigger"),
		}
	case "istest":
		return map[string]string{
			"classes/" + row.id + ".cls":          "@IsTest\nprivate class " + row.id + " {\n  @IsTest static void run() {\n    " + body + "\n  }\n}\n",
			"classes/" + row.id + ".cls-meta.xml": meta("ApexClass"),
		}
	}
	return map[string]string{
		"classes/" + name + ".cls":          "public class " + name + " {\n  public static void run() {\n    " + body + "\n  }\n}\n",
		"classes/" + name + ".cls-meta.xml": meta("ApexClass"),
	}
}

// unknownFieldOrgShape is the feature set of the capture org, as the project
// would declare it. The surface orgs had MultiCurrency, so CurrencyIsoCode
// compiled there; a project without the feature does not have the field.
func unknownFieldOrgShape(org string) string {
	switch org {
	case "case-user-surface":
		return "org:\n  features: [MultiCurrency, Entitlements, PersonAccounts, StateAndCountryPicklist]\n"
	case "webstore-surface":
		return "org:\n  features: [MultiCurrency, Entitlements, PersonAccounts, StateAndCountryPicklist, OrderManagement]\n"
	}
	return ""
}

func unknownFieldErrors(diagnostics []diagnostic.Diagnostic) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	for _, d := range diagnostics {
		if d.Severity == diagnostic.Error {
			out = append(out, d)
		}
	}
	return out
}

func unknownFieldMessages(diagnostics []diagnostic.Diagnostic) []string {
	var out []string
	for _, d := range diagnostics {
		message := d.NativeMessage
		if message == "" {
			message = d.Message
		}
		out = append(out, d.Code+": "+message)
	}
	return out
}

func unknownFieldRowSkipped(row unknownFieldRow) string {
	// SOQL text is another surface; the FLS-hidden standard fields and the
	// no-FLS custom field differ between routes and belong to the FLS rows.
	if strings.Contains(row.problem, "SELECT") || strings.HasPrefix(row.problem, "\n") {
		return "SOQL text surface"
	}
	if row.route == "anonymous" && (strings.HasPrefix(row.id, "H_") || strings.HasPrefix(row.id, "F_") || strings.HasPrefix(row.id, "K_cont_GenderIdentity") || strings.HasPrefix(row.id, "K_cont_Pronouns")) {
		return "anonymous FLS visibility (case_standard_field_fls)"
	}
	if row.org == "plain-granted" && row.route == "anonymous" {
		return "anonymous FLS visibility (case_standard_field_fls)"
	}
	if strings.Contains(row.problem, "is not writeable") {
		return "read-only field rules (case_standard_field_fls)"
	}
	if strings.HasPrefix(row.id, "W_") {
		return "local class declaration rows run through the CLI route"
	}
	return ""
}

func TestUnknownSObjectFieldNativeRows(t *testing.T) {
	rows, bodies := readUnknownFieldRows(t)
	type group struct{ org, api, route string }
	groups := map[group][]unknownFieldRow{}
	for _, row := range rows {
		groups[group{row.org, row.api, row.route}] = append(groups[group{row.org, row.api, row.route}], row)
	}
	var keys []group
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].org+keys[i].api+keys[i].route < keys[j].org+keys[j].api+keys[j].route
	})
	for _, key := range keys {
		projectDir := "project"
		if key.org == "namespaced" {
			projectDir = "project_namespaced"
		}
		groupRows := groups[key]
		t.Run(key.org+"/"+key.api+"/"+key.route, func(t *testing.T) {
			// Each named row gets its own index: an SObject literal in one
			// class infers its fields onto the object for the whole project.
			var anonymousIndex typesys.Index
			if key.route == "anonymous" {
				anonymousIndex = unknownFieldIndex(t, projectDir, nil)
			}
			for _, row := range groupRows {
				if reason := unknownFieldRowSkipped(row); reason != "" {
					continue
				}
				if reason := unknownFieldParkedReason(row); reason != "" {
					continue
				}
				t.Run(row.id, func(t *testing.T) {
					var got []diagnostic.Diagnostic
					if key.route != "anonymous" {
						sources := unknownFieldSources(row, bodies[row.id])
						if shape := unknownFieldOrgShape(key.org); shape != "" {
							sources["/glade.yml"] = shape
						}
						index := unknownFieldIndex(t, projectDir, sources)
						got = unknownFieldErrors(Analyze(index).Diagnostics)
					} else {
						got = unknownFieldErrors(AnalyzeAnonymous(anonymousIndex, bodies[row.id], key.api).Diagnostics)
					}
					if row.compiled {
						if len(got) != 0 {
							t.Fatalf("native compiled %s; glade rejected: %v", row.id, unknownFieldMessages(got))
						}
						return
					}
					matched := 0
					for _, d := range got {
						if d.NativeMessage == row.problem || d.Message == row.problem {
							matched++
						}
					}
					if matched > 0 && (row.problemCount == 0 || matched == row.problemCount) {
						return
					}
					if matched > 0 {
						t.Fatalf("native reported %q %d times; glade %d times: %v", row.problem, row.problemCount, matched, unknownFieldMessages(got))
					}
					t.Fatalf("native rejected with %q; glade diagnostics: %v", row.problem, unknownFieldMessages(got))
				})
			}
		})
	}
}

// A name in another namespace stays open on a standard object: Salesforce
// accepts it only for an installed package, and a project that declares no
// package dependency cannot say which are installed. The rows
// N_*_asgn/read in org.tsv (no package installed) are parked for this reason;
// this test keeps the tolerance from regressing.
func TestUnknownSObjectFieldOtherNamespaceStaysOpen(t *testing.T) {
	const body = "Account a = new Account(); a.fake__X__c = 1; Object v = a.fake__X__c; Contact c = new Contact(); c.Account.fake__X__c = 1;"
	for _, api := range []string{"62.0", "67.0"} {
		row := unknownFieldRow{api: api, route: "named", id: "tolerance"}
		index := unknownFieldIndex(t, "project", unknownFieldSources(row, body))
		if got := unknownFieldErrors(Analyze(index).Diagnostics); len(got) != 0 {
			t.Fatalf("named %s: %v", api, unknownFieldMessages(got))
		}
		if got := unknownFieldErrors(AnalyzeAnonymous(unknownFieldIndex(t, "project", nil), body, api).Diagnostics); len(got) != 0 {
			t.Fatalf("anonymous %s: %v", api, unknownFieldMessages(got))
		}
	}
}
