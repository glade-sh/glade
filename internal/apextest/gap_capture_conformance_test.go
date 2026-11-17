package apextest

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// Replay of native capture families under testdata/conformance/<family>/:
//
//	cases.tsv           id, route, file[, api]; no api column means both 62.0 and 67.0
//	apex/<file>         anonymous source, run in table order on one org per API
//	project/force-app   the deployed metadata; apiVersion is rewritten per API
//	observations.jsonl  native rows: OBS debug lines or per-method test outcomes
//
// A row with "alt" lists the observations of the repeat run. Rows whose order
// differs between repeats are compared as a multiset (see gapFamily.unordered).
type gapObservation struct {
	ID    string      `json:"id"`
	API   string      `json:"api"`
	Route string      `json:"route"`
	Debug []string    `json:"debug"`
	Error string      `json:"error"`
	Tests [][3]string `json:"tests"`
	Alt   []struct {
		Debug []string `json:"debug"`
	} `json:"alt"`
}

type gapCase struct{ id, route, file, api string }

type gapFamily struct {
	dir  string
	apis []string
	// residual lists rows that do not match yet, by "id/api". A listed row must
	// still differ; when it matches the test fails until the entry is removed.
	residual map[string]string
	// unordered lists rows whose order differs between native repeats. Their
	// OBS payload is compared as a set of comma-separated values.
	unordered map[string]bool
	// limited lists rows that return the first N values of an unstable order;
	// they must be N distinct values.
	limited map[string]int
	// normalize masks a debug payload that the family does not claim.
	normalize func(id, line string) string
	// adminFrom names permission sets whose object and field grants the local
	// Admin profile carries. The native @IsTest rows run as the capture user,
	// who held the same permission set (the rows' fls case assigns it).
	adminFrom []string
	// classAPI maps an API under test to the apiVersion written into the class
	// metadata, for rows whose capture did not prove the class API.
	classAPI map[string]string
	// cleanBetween empties the custom objects after each anonymous case, as
	// the capture's per-case residue cleanup did.
	cleanBetween bool
}

var gapIDPattern = regexp.MustCompile(`\b(?:001|003|005|006|00D|00G|00Q|00e|01p|01q|0Hp|0HM|707|a0[0-9A-Za-z])[0-9A-Za-z]{12}(?:[0-9A-Za-z]{3})?\b`)

func maskGapIDs(s string) string { return gapIDPattern.ReplaceAllString(s, "<id>") }

func (f gapFamily) readObservations(t *testing.T) map[string]gapObservation {
	t.Helper()
	file, err := os.Open(filepath.Join(f.dir, "observations.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	out := map[string]gapObservation{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1<<20), 1<<24)
	for scanner.Scan() {
		var row gapObservation
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			t.Fatal(err)
		}
		key := row.ID + "/" + row.API
		if _, dup := out[key]; dup {
			t.Fatalf("duplicate native row %s", key)
		}
		if gapIDPattern.MatchString(strings.Join(row.Debug, "\n")) || gapIDPattern.MatchString(row.Error) {
			t.Fatalf("native row %s carries an unmasked id", key)
		}
		out[key] = row
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func (f gapFamily) readCases(t *testing.T) []gapCase {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(f.dir, "cases.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	var out []gapCase
	for i, line := range strings.Split(strings.TrimSpace(string(body)), "\n") {
		if i == 0 {
			continue
		}
		cells := strings.Split(line, "\t")
		if len(cells) < 3 || len(cells) > 4 {
			t.Fatalf("bad case row %q", line)
		}
		c := gapCase{id: cells[0], route: cells[1], file: cells[2]}
		if len(cells) == 4 {
			c.api = cells[3]
		}
		out = append(out, c)
	}
	return out
}

var gapAPIVersionPattern = regexp.MustCompile(`<apiVersion>[^<]*</apiVersion>`)

func (f gapFamily) writeProject(t *testing.T, api string) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
	source := filepath.Join(f.dir, "project", "force-app")
	err := filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		text := string(body)
		if strings.HasSuffix(path, "-meta.xml") && strings.Contains(text, "<apiVersion>") {
			version := api
			if override, ok := f.classAPI[api]; ok && strings.Contains(filepath.ToSlash(rel), "/classes/") {
				version = override
			}
			text = gapAPIVersionPattern.ReplaceAllString(text, "<apiVersion>"+version+"</apiVersion>")
		}
		writeFile(t, filepath.Join(root, "force-app", rel), text)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.adminFrom) > 0 {
		writeFile(t, filepath.Join(root, "force-app/main/default/profiles/Admin.profile-meta.xml"), f.adminProfile(t))
	}
	return root
}

var gapPermissionBlock = regexp.MustCompile(`(?s)<(?:fieldPermissions|objectPermissions)>.*?</(?:fieldPermissions|objectPermissions)>`)

// adminProfile builds an Admin profile from the grants of f.adminFrom, with
// object access widened to View All and Modify All.
func (f gapFamily) adminProfile(t *testing.T) string {
	t.Helper()
	var body strings.Builder
	body.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n" + `<Profile xmlns="http://soap.sforce.com/2006/04/metadata"><custom>false</custom>`)
	for _, name := range f.adminFrom {
		source, err := os.ReadFile(filepath.Join(f.dir, "project", "force-app", "main", "default", "permissionsets", name+".permissionset-meta.xml"))
		if err != nil {
			t.Fatal(err)
		}
		for _, block := range gapPermissionBlock.FindAllString(string(source), -1) {
			block = strings.NewReplacer("<modifyAllRecords>false", "<modifyAllRecords>true", "<viewAllRecords>false", "<viewAllRecords>true").Replace(block)
			body.WriteString(strings.Join(strings.Fields(block), " "))
		}
	}
	body.WriteString("</Profile>\n")
	return body.String()
}

func gapSplit(payload string) []string {
	_, rest, _ := strings.Cut(payload, "|")
	_, rest, _ = strings.Cut(rest, "|")
	if rest == "" {
		return nil
	}
	values := strings.Split(rest, ",")
	sort.Strings(values)
	return values
}

func (f gapFamily) replay(t *testing.T) {
	releaseRuntimeCachesAfterTest(t)
	cases := f.readCases(t)
	rows := f.readObservations(t)
	apis := f.apis
	if len(apis) == 0 {
		apis = []string{"62.0", "67.0"}
	}
	for _, api := range apis {
		api := api
		t.Run(api, func(t *testing.T) {
			var mine []gapCase
			for _, c := range cases {
				if c.api == "" || c.api == api {
					mine = append(mine, c)
				}
			}
			if len(mine) == 0 {
				t.Skip("no rows at this API")
			}
			root := f.writeProject(t, api)
			index := loadTestIndex(t, root)
			check := func(t *testing.T, c gapCase, got []string, gotErr string) {
				t.Helper()
				want := rows[c.id+"/"+api]
				if want.ID == "" {
					t.Fatalf("missing native row %s/%s", c.id, api)
				}
				matched := f.matches(c.id, want, got, gotErr)
				residual, isResidual := f.residual[c.id+"/"+api]
				if !isResidual {
					residual, isResidual = f.residual[c.id+"/*"]
				}
				switch {
				case isResidual && matched:
					t.Fatalf("%s %s now matches native; remove it from the residual ledger", c.id, api)
				case isResidual:
					t.Skipf("known residual: %s\nnative=%q err=%q\nglade =%q err=%q", residual, want.Debug, want.Error, got, gotErr)
				case !matched:
					t.Fatalf("NATIVE %s %s\nnative=%q err=%q\nglade =%q err=%q", c.id, api, want.Debug, want.Error, got, gotErr)
				}
			}
			var anonymous []gapCase
			for _, c := range mine {
				if c.route == "istest" {
					c := c
					t.Run(c.id, func(t *testing.T) {
						run := Run(index, Options{SelectedClasses: []string{c.file}, NoDiskCache: true, Parallelism: 1})
						var got []string
						for _, suite := range run.Suites {
							for _, entry := range suite.Cases {
								outcome := "Pass"
								text := ""
								if entry.Problem != nil {
									outcome = "Fail"
									text = entry.Problem.Type + ": " + entry.Problem.Message
								}
								got = append(got, entry.MethodName+"|"+outcome+"|"+maskGapIDs(text))
							}
						}
						sort.Strings(got)
						check(t, c, got, "")
					})
					continue
				}
				anonymous = append(anonymous, c)
			}
			if len(anonymous) == 0 {
				return
			}
			t.Run("anonymous", func(t *testing.T) {
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org, LinkProject: true})
				triggers, triggerErrors := compileProjectTriggers(index)
				if len(triggerErrors) != 0 {
					t.Fatalf("trigger compile: %v", triggerErrors)
				}
				for _, trigger := range triggers {
					if err := runner.base.RegisterTrigger(trigger); err != nil {
						t.Fatal(err)
					}
				}
				machine := runner.newMachine()
				machine.SetToolingExecuteAnonymous(true)
				machine.SetLimitMode(vm.LimitModeStrict)
				for _, c := range anonymous {
					c := c
					t.Run(c.id, func(t *testing.T) {
						body, err := os.ReadFile(filepath.Join(f.dir, "apex", c.file))
						if err != nil {
							t.Fatal(err)
						}
						src := string(body)
						if analysis := sema.AnalyzeAnonymous(index, src, api); analysis.HasErrors() {
							t.Fatalf("native accepted anonymous: %v", analysis.Diagnostics)
						}
						program, err := vm.CompileAnonymousWithOptions(src, vm.CompileOptions{APIVersion: api, ApprovedPrefixStatements: sema.ApprovedAnonymousPrefixStatements(index, src, api)})
						if err != nil {
							t.Fatal(err)
						}
						// Each native anonymous call is its own transaction: fresh
						// governor counters and static state, the org persists.
						machine.ResetLimits()
						if err := machine.ResetStatics(); err != nil {
							t.Fatal(err)
						}
						result, err := machine.Execute(program)
						gotErr := ""
						if err != nil {
							var runtimeErr *vm.RuntimeError
							if !errors.As(err, &runtimeErr) {
								t.Fatal(err)
							}
							gotErr = runtimeErr.Type + ": " + runtimeErr.ExceptionMessage()
						}
						var got []string
						for _, line := range result.Debug {
							if _, rest, ok := strings.Cut(line, "OBS|"); ok {
								got = append(got, maskGapIDs("OBS|"+rest))
							}
						}
						check(t, c, got, maskGapIDs(gotErr))
						if f.cleanBetween {
							for name, state := range machine.Org.Objects {
								if strings.HasSuffix(strings.ToLower(name), "__c") {
									state.Records = nil
									machine.Org.Objects[name] = state
								}
							}
						}
					})
				}
			})
		})
	}
}

func (f gapFamily) matches(id string, want gapObservation, got []string, gotErr string) bool {
	if want.Tests != nil {
		var wantRows []string
		for _, test := range want.Tests {
			wantRows = append(wantRows, test[0]+"|"+test[1]+"|"+test[2])
		}
		sort.Strings(wantRows)
		return strings.Join(wantRows, "\n") == strings.Join(got, "\n")
	}
	trim := func(s string) string { return strings.TrimPrefix(strings.TrimRight(s, " :"), "System.") }
	if trim(want.Error) != trim(gotErr) {
		return false
	}
	if len(want.Debug) != len(got) {
		return false
	}
	for i := range got {
		wantLine, gotLine := want.Debug[i], got[i]
		if f.normalize != nil {
			wantLine, gotLine = f.normalize(id, wantLine), f.normalize(id, gotLine)
		}
		switch {
		case f.unordered[id]:
			if strings.Join(gapSplit(wantLine), ",") != strings.Join(gapSplit(gotLine), ",") {
				return false
			}
		case f.limited[id] > 0:
			values := gapSplit(gotLine)
			seen := map[string]bool{}
			for _, value := range values {
				seen[value] = true
			}
			if len(values) != f.limited[id] || len(seen) != len(values) {
				return false
			}
		default:
			if wantLine != gotLine {
				return false
			}
		}
	}
	return true
}

var _ = fmt.Sprintf
