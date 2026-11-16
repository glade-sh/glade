package apextest

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// The API 68 capture replays one scratch org per feature shape. Families ran
// in capture order against the same org, so deployed metadata and record data
// accumulate across families exactly as families.tsv orders them. Each row's
// native outcome is the org.tsv outcome column; the trailing debug-log limits
// summary is a harness addition and is not compared.
const api68NativeRoot = "testdata/conformance/api68_native"

// api68NativeGaps lists the rows whose native behavior Glade does not model.
// A listed row that starts to match fails the test so the list stays exact.
var api68NativeGaps = map[string]string{
	"limits/LIMITS-ORGLIMITS-68":      "OrgLimits keys and daily usage are org history; Glade models 19 keys sized from transaction caps",
	"limits/LIMITS-ORGLIMITS-67":      "OrgLimits keys and daily usage are org history; Glade models 19 keys sized from transaction caps",
	"pdf/PDF-TAGGED-68":               "Visualforce PDF bytes: Glade renders its own PDF, not the native 1139-byte document",
	"pdf/PDF-TAGGED-67":               "Visualforce PDF bytes: Glade renders its own PDF, not the native 1139-byte document",
	"pdf/PDF-MARKERS-68":              "ContentVersion byte scan through REST is not a Glade route",
	"pdf/PDF-MARKERS-67":              "ContentVersion byte scan through REST is not a Glade route",
	"elastic/ELASTIC-ENQ-68":          "OrgLimits DailyAsyncApexExecutions usage and ceiling are org history and org limits",
	"elastic/ELASTIC-ENQ-67":          "OrgLimits DailyAsyncApexExecutions usage and ceiling are org history and org limits",
	"elastic/ELASTIC-ENQ-OVERRIDE-68": "OrgLimits DailyAsyncApexExecutions usage and ceiling are org history and org limits",
	"elastic/ELASTIC-READ":            "native batch jobs interleave by CreatedDate; Glade drains each job in order",
	"elastic-250/ELASTIC-250-LIMITS":  "asyncApexExecutionsOverride and daily usage are org limits Glade does not model",
	"elastic-250/ELASTIC-250-ENQ":     "asyncApexExecutionsOverride and daily usage are org limits Glade does not model",
	"heap/HEAP-SYNC-68":               "Limits.getHeapSize bytes are not byte-exact in Glade",
	"heap/HEAP-SYNC-67":               "Limits.getHeapSize bytes are not byte-exact in Glade",
	"heap/HEAP-SYNC-OVERRIDE-68":      "Limits.getHeapSize bytes are not byte-exact in Glade",
	"heap/HEAP-READ":                  "async getHeapSize bytes; the native org also held one more async68 row than the case sequence enqueued",
	"heap/HEAP-READ-OVERRIDE":         "async getHeapSize bytes are not byte-exact in Glade",
	"sharing-on/SHARE-ON-PERSISTED":   "Metadata API settings retrieve is not a Glade route",
	"schema-pe/PE-SCHEMA-68":          "describe: UniversalAnomalyEvent is missing from the generated standard catalog; FlowOrchestrationWorkItemEvent reports permission-derived createable/queryable and sorted field keys instead of the bundle's object flags and field order",
	"schema-pe/PE-SCHEMA-67":          "describe: UniversalAnomalyEvent is missing from the generated standard catalog",
}

// api68NativePartialGaps compare a row after removing the one segment Glade
// does not model; the rest of the row must match exactly.
var api68NativePartialGaps = map[string]struct {
	segment *regexp.Regexp
	reason  string
}{
	"sharing-off/SHARE-OFF-68": {api68CustomRowCauseSegment, api68CustomRowCauseReason},
	"sharing-off/SHARE-OFF-67": {api68CustomRowCauseSegment, api68CustomRowCauseReason},
	"sharing-on/SHARE-ON-68":   {api68CustomRowCauseSegment, api68CustomRowCauseReason},
	"sharing-on/SHARE-ON-67":   {api68CustomRowCauseSegment, api68CustomRowCauseReason},
}

// The customReason test method logs first (methods run in name order).
var api68CustomRowCauseSegment = regexp.MustCompile(`^CUSTOM_[A-Z]+=.*?;(MANUAL_SHARES=)`)

const api68CustomRowCauseReason = "AccountShare RowCause accepts Capture__c: Glade has no AccountShare RowCause value set to reject it"

type api68NativeCase struct {
	family, id, api, route, phase, variant, source string
	native                                         string
}

type api68NativeOrg struct {
	t          *testing.T
	scratch    string
	deployed   map[string]string
	generation int
	index      typesys.Index
	indexGen   int
	runner     *conformanceRunner
	machine    *vm.VM
	runnerGen  int
	data       *storage.OrgState
}

func TestAPI68NativeCaptureConformance(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	scratch, err := os.ReadFile(filepath.Join(api68NativeRoot, "project-scratch-def.json"))
	if err != nil {
		t.Fatal(err)
	}
	org := &api68NativeOrg{t: t, scratch: string(scratch), deployed: map[string]string{}, generation: 1}
	var mismatches []string
	matched, total := 0, 0
	seenGaps := map[string]bool{}
	for _, family := range api68NativeRows(t, "families.tsv")[1:] {
		name, permset := family[1], family[4]
		cases := api68NativeFamilyCases(t, name)
		// The capture deploys every upfront variant in plan order, assigns the
		// family permission set, then runs the cases; deploy rows report the
		// upfront result. "after" rows deploy when they are reached.
		upfront := map[string]string{}
		for _, variant := range api68NativeRows(t, filepath.Join(name, "variants.tsv"))[1:] {
			if variant[2] == "true" {
				upfront[variant[0]] = org.deploy(name, variant[0], api68NativeVariantOutcome(cases, variant[0]))
			}
		}
		if permset != "-" {
			org.assignPermissionSet(permset)
		}
		for _, row := range cases {
			total++
			var got string
			switch {
			case row.route == "deploy" && row.phase == "case":
				got = upfront[row.variant]
			case row.route == "deploy":
				got = org.deploy(name, row.variant, row.native)
			default:
				got = org.run(row)
			}
			want := api68NormalizeNative(row.native)
			got = api68NormalizeGlade(got)
			key := name + "/" + row.id
			if partial, ok := api68NativePartialGaps[key]; ok {
				if !partial.segment.MatchString(want) || !partial.segment.MatchString(got) {
					t.Errorf("%s partial gap segment is missing\n  native: %s\n  glade:  %s", key, want, got)
				}
				want = partial.segment.ReplaceAllString(want, "$1")
				got = partial.segment.ReplaceAllString(got, "$1")
			}
			_, gap := api68NativeGaps[key]
			if gap {
				seenGaps[key] = true
			}
			if got == want {
				matched++
				if gap {
					t.Errorf("%s now matches native; remove it from api68NativeGaps", key)
				}
				continue
			}
			mismatches = append(mismatches, fmt.Sprintf("%s\n  native: %s\n  glade:  %s", key, want, got))
			if !gap {
				t.Errorf("%s (%s %s)\n  native: %s\n  glade:  %s", key, row.route, row.api, want, got)
			}
		}
	}
	for key := range api68NativeGaps {
		if !seenGaps[key] {
			t.Errorf("api68NativeGaps names unknown row %s", key)
		}
	}
	t.Logf("API 68 native capture: %d/%d rows match", matched, total)
	if os.Getenv("GLADE_API68_REPORT") != "" {
		t.Logf("mismatches:\n%s", strings.Join(mismatches, "\n"))
	}
}

// api68NativeVariantOutcome is the native deploy result of a variant: its
// case row, else its after-phase row.
func api68NativeVariantOutcome(cases []api68NativeCase, variant string) string {
	outcome := ""
	for _, row := range cases {
		if row.route == "deploy" && row.variant == variant {
			if row.phase == "case" {
				return row.native
			}
			outcome = row.native
		}
	}
	return outcome
}

func api68NativeRows(t *testing.T, rel string) [][]string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(api68NativeRoot, rel))
	if err != nil {
		t.Fatal(err)
	}
	var rows [][]string
	for _, line := range strings.Split(strings.TrimRight(string(body), "\n"), "\n") {
		rows = append(rows, strings.Split(line, "\t"))
	}
	return rows
}

func api68NativeFamilyCases(t *testing.T, family string) []api68NativeCase {
	t.Helper()
	native := map[string]string{}
	for _, row := range api68NativeRows(t, filepath.Join(family, "org.tsv"))[1:] {
		if len(row) != 6 {
			t.Fatalf("%s org.tsv row has %d columns", family, len(row))
		}
		if _, duplicate := native[row[0]]; duplicate {
			t.Fatalf("%s org.tsv repeats %s", family, row[0])
		}
		native[row[0]] = row[5]
	}
	var cases []api68NativeCase
	for _, row := range api68NativeRows(t, filepath.Join(family, "cases.tsv"))[1:] {
		outcome, ok := native[row[0]]
		if !ok {
			t.Fatalf("%s case %s has no native row", family, row[0])
		}
		delete(native, row[0])
		cases = append(cases, api68NativeCase{family: family, id: row[0], api: row[1], route: row[2], phase: row[3], variant: row[4], source: row[5], native: outcome})
	}
	if len(native) != 0 {
		t.Fatalf("%s org.tsv rows without a case: %v", family, native)
	}
	return cases
}

var (
	api68LimitsSuffix = regexp.MustCompile(` \[LIMITS [^\]]*\]$`)
	api68IDToken      = regexp.MustCompile(`\b[a-zA-Z0-9]{15}(?:[a-zA-Z0-9]{3})?\b`)
)

// The capture masks record ids as <id> and 00DREDACTED; Glade prints real ids.
func api68MaskIDs(text string) string {
	text = strings.ReplaceAll(text, "00DREDACTED", "<id>")
	return api68IDToken.ReplaceAllStringFunc(text, func(token string) string {
		if strings.ContainsAny(token, "0123456789") {
			return "<id>"
		}
		return token
	})
}

func api68NormalizeNative(outcome string) string {
	outcome = api68LimitsSuffix.ReplaceAllString(outcome, "")
	outcome = strings.TrimPrefix(outcome, "Execution failed at this code: ")
	return api68MaskIDs(outcome)
}

func api68NormalizeGlade(outcome string) string {
	return api68MaskIDs(outcome)
}

func (o *api68NativeOrg) variantFiles(family, variant string) map[string]string {
	t := o.t
	t.Helper()
	root := filepath.Join(api68NativeRoot, family, "src", variant, "force-app", "main", "default")
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		files[filepath.ToSlash(rel)] = string(body)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// build writes one org generation as a project. The scratch definition carries
// the capture org's edition and features.
func (o *api68NativeOrg) build(files map[string]string) typesys.Index {
	t := o.t
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"68.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "config", "project-scratch-def.json"), o.scratch)
	for rel, body := range files {
		writeFile(t, filepath.Join(root, "force-app", "main", "default", filepath.FromSlash(rel)), body)
	}
	return loadTestIndex(t, root)
}

func (o *api68NativeOrg) current() typesys.Index {
	if o.indexGen != o.generation {
		o.index = o.build(o.deployed)
		o.indexGen = o.generation
	}
	return o.index
}

// session returns the machine for the current metadata generation. Records the
// previous generation committed carry over, as they do in the org.
func (o *api68NativeOrg) session() (*conformanceRunner, *vm.VM) {
	if o.runnerGen == o.generation && o.machine != nil {
		return o.runner, o.machine
	}
	index := o.current()
	org := orgFromIndex(index)
	org.APIVersion = "68.0"
	if o.data == nil {
		api68SeedCaptureOrg(o.t, &org)
	} else {
		for name, state := range o.data.Objects {
			next, ok := org.Objects[name]
			if !ok {
				continue
			}
			for id, record := range state.Records {
				next.Records[id] = record
			}
			org.Objects[name] = next
		}
		// Carried records keep their ids; new inserts continue after them.
		if org.IDSequences == nil {
			org.IDSequences = map[string]uint64{}
		}
		for key, value := range o.data.IDSequences {
			if value > org.IDSequences[key] {
				org.IDSequences[key] = value
			}
		}
	}
	o.runner = newConformanceRunner(o.t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
	o.machine = o.runner.newMachine()
	o.machine.SetToolingExecuteAnonymous(true)
	o.data = o.machine.Org
	o.runnerGen = o.generation
	return o.runner, o.machine
}

// api68SeedCaptureOrg installs the records the capture org held before the
// first family: LIMITS-CURSOR-* counted one Account (NUM=1, CURSOR_ROWS=1),
// and RUN-FACTS names the Standard User the sharing and flow tests query by
// alias.
func api68SeedCaptureOrg(t *testing.T, org *storage.OrgState) {
	t.Helper()
	accounts := org.Objects["Account"]
	accountID := storage.ID("001000000000681AAA")
	accounts.Records[accountID] = storage.Record{ID: accountID, Object: "Account", Fields: map[string]storage.Value{"Id": storage.IDValue(accountID), "Name": storage.StringValue("Capture org account"), "OwnerId": storage.IDValue(storage.DefaultUserID)}}
	org.Objects["Account"] = accounts
	var profile storage.ID
	for id, p := range org.Objects["Profile"].Records {
		if p.Fields["Name"].String == "Standard User" {
			profile = id
			break
		}
	}
	if profile == "" {
		t.Fatal("Standard User profile missing")
	}
	users := org.Objects["User"]
	userID := storage.ID("005000000000681AAA")
	fields := map[string]storage.Value{}
	for name, value := range users.Records[storage.DefaultUserID].Fields {
		fields[name] = value
	}
	fields["Id"] = storage.IDValue(userID)
	fields["Alias"] = storage.StringValue("cap68u1")
	fields["Username"] = storage.StringValue("cap68u1@example.invalid")
	fields["LastName"] = storage.StringValue("Capture")
	fields["ProfileId"] = storage.IDValue(profile)
	fields["IsActive"] = storage.BooleanValue(true)
	fields["UserRoleId"] = storage.NullValue()
	users.Records[userID] = storage.Record{ID: userID, Object: "User", Fields: fields}
	org.Objects["User"] = users
}

func (o *api68NativeOrg) run(row api68NativeCase) string {
	switch row.route {
	case "anonymous":
		return o.anonymous(row)
	case "test", "testasync":
		return o.test(row)
	default:
		// Settings retrieve and ContentVersion byte scans are not Glade routes.
		return "route not modeled: " + row.source
	}
}

var api68NativeComponent = regexp.MustCompile(`^[A-Za-z]+ [A-Za-z0-9_.]+: `)

// Native deploy failures list "<Type> <Name>: <problem>" joined by "; ".
// Warnings print "deployed; WARN[code] ... SEVERE [l:c]: <problem>".
func api68NativeDeployMessages(outcome string) []string {
	var out []string
	for _, part := range strings.Split(outcome, "; ") {
		if loc := api68NativeComponent.FindStringIndex(part); loc != nil {
			out = append(out, part[loc[1]:])
		} else if len(out) > 0 {
			out[len(out)-1] += "; " + part
		}
	}
	return out
}

var api68NativeWarning = regexp.MustCompile(`SEVERE \[\d+:\d+\]: (.*)$`)

func (o *api68NativeOrg) deploy(family, name, nativeOutcome string) string {
	t := o.t
	t.Helper()
	variant := o.variantFiles(family, name)
	candidate := map[string]string{}
	for rel, body := range o.deployed {
		candidate[rel] = body
	}
	for rel, body := range variant {
		candidate[rel] = body
	}
	index := o.build(candidate)
	diagnostics := append([]diagnostic.Diagnostic(nil), index.Diagnostics...)
	diagnostics = append(diagnostics, sema.Analyze(index).Diagnostics...)
	errorSet, warnings := map[string]bool{}, map[string]bool{}
	for _, d := range diagnostics {
		file := filepath.ToSlash(d.File)
		owned := false
		for rel := range variant {
			if strings.HasSuffix(file, "/force-app/main/default/"+rel) {
				owned = true
				break
			}
		}
		if !owned {
			continue
		}
		message := d.Message
		if d.NativeMessage != "" {
			message = d.NativeMessage
		}
		switch {
		case d.Severity == diagnostic.Error:
			errorSet[message] = true
		case d.Severity == diagnostic.Warning && d.NativeMessage != "":
			warnings[d.NativeMessage] = true
		}
	}
	native := api68NormalizeNative(nativeOutcome)
	if strings.HasPrefix(native, "deployed") {
		// The org follows the native outcome so one rejected deploy does not
		// turn every later row of the family into the same mismatch.
		o.deployed = candidate
		o.generation++
		if len(errorSet) > 0 {
			return "rejected: " + strings.Join(api68Sorted(errorSet), "; ")
		}
		want := ""
		if match := api68NativeWarning.FindStringSubmatch(native); match != nil {
			want = match[1]
		}
		got := strings.Join(api68Sorted(warnings), "; ")
		if got != want {
			return "deployed; warnings=" + got
		}
		return native
	}
	if len(errorSet) == 0 {
		return "deployed"
	}
	// Native deploys report each problem per occurrence and stop at the
	// first failing phase. Compare that every native problem is reported.
	for _, message := range api68NativeDeployMessages(native) {
		if !errorSet[message] {
			return "rejected: " + strings.Join(api68Sorted(errorSet), "; ")
		}
	}
	return native
}

func api68Sorted(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func api68RuntimeErrorText(err error) string {
	var runtimeErr *vm.RuntimeError
	if !errors.As(err, &runtimeErr) {
		return "error: " + err.Error()
	}
	typ := runtimeErr.Type
	if !strings.Contains(typ, ".") {
		typ = "System." + typ
	}
	return typ + ": " + runtimeErr.ExceptionMessage()
}

func (o *api68NativeOrg) anonymous(row api68NativeCase) string {
	t := o.t
	t.Helper()
	body, err := os.ReadFile(filepath.Join(api68NativeRoot, row.family, row.source))
	if err != nil {
		t.Fatal(err)
	}
	source := string(body)
	_, machine := o.session()
	if analysis := sema.AnalyzeAnonymous(o.current(), source, row.api); analysis.HasErrors() {
		var messages []string
		for _, d := range analysis.Diagnostics {
			if d.Severity == diagnostic.Error {
				messages = append(messages, d.Message)
			}
		}
		return "compile: " + strings.Join(messages, "; ")
	}
	program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: row.api})
	if err != nil {
		return "compile: " + err.Error()
	}
	// Each anonymous request is its own transaction with fresh governor limits.
	machine.ResetLimits()
	result, runErr := machine.Execute(program)
	lines := append([]string(nil), result.Debug...)
	// Native async jobs run after the anonymous transaction, in their own logs.
	var async vm.Result
	if err := machine.DrainAsync(&async); err != nil {
		t.Logf("%s async drain: %v", row.id, err)
	}
	if len(lines) > 0 {
		return strings.Join(lines, ";")
	}
	if runErr != nil {
		return api68RuntimeErrorText(runErr)
	}
	return "ok"
}

func (o *api68NativeOrg) test(row api68NativeCase) string {
	t := o.t
	t.Helper()
	runner, _ := o.session()
	index := o.current()
	cases := Discover(index, Options{SelectedClasses: []string{row.source}})
	if len(cases) == 0 {
		return "no test methods discovered for " + row.source
	}
	methods, methodErrors := compileTestMethods(cases)
	programs, programErrors := compileTestInvokePrograms(cases)
	runtimeMethods := indexTestRuntimeMethods(methods)
	counters := newRunPerfCounters(false)
	debug := map[string][]string{}
	var problems []string
	opts := Options{NoDiskCache: true, Parallelism: 1, debugSink: func(class, method string, lines []string) {
		debug[method] = append([]string(nil), lines...)
	}}
	for _, testCase := range cases {
		key := testCaseKey(testCase)
		seed := o.data.CloneRuntime()
		initializeTestOrg(&seed)
		result := runCase(context.Background(), testCase, methods[key], runtimeMethods[testMethodSourceKey(testCase.ClassName, testCase.File)], methodErrors[key], programs[key], programErrors[key], runner.base, nil, nil, nil, seed, 0, opts, false, nil, counters)
		if result.Problem != nil {
			problems = append(problems, testCase.MethodName+": "+result.Problem.Type+": "+result.Problem.Message)
		}
	}
	// The capture log lists the test methods in name order.
	names := make([]string, 0, len(debug))
	for name := range debug {
		names = append(names, name)
	}
	sort.Strings(names)
	var lines []string
	for _, name := range names {
		lines = append(lines, debug[name]...)
	}
	out := strings.Join(lines, ";")
	if len(problems) > 0 {
		out += " FAILED[" + strings.Join(problems, "; ") + "]"
	}
	return out
}

// assignPermissionSet mirrors the capture's permission set assignment to the
// running user after the family's upfront deploys.
func (o *api68NativeOrg) assignPermissionSet(name string) {
	t := o.t
	t.Helper()
	_, machine := o.session()
	org := machine.Org
	var setID storage.ID
	for id, record := range org.Objects["PermissionSet"].Records {
		if strings.EqualFold(record.Fields["Name"].String, name) {
			setID = id
			break
		}
	}
	if setID == "" {
		t.Fatalf("permission set %s is not deployed", name)
	}
	storage.EnsureStandardObject(org, "PermissionSetAssignment")
	assignments := org.Objects["PermissionSetAssignment"]
	assignmentID := storage.ID(fmt.Sprintf("0Pa00000000%04dAAA", len(assignments.Records)+681))
	assignments.Records[assignmentID] = storage.Record{ID: assignmentID, Object: "PermissionSetAssignment", Fields: map[string]storage.Value{
		"Id":              storage.IDValue(assignmentID),
		"AssigneeId":      storage.IDValue(storage.DefaultUserID),
		"PermissionSetId": storage.IDValue(setID),
	}}
	org.Objects["PermissionSetAssignment"] = assignments
}
