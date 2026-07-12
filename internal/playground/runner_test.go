package playground

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestRunnerExecutesAnonymousAgainstWorkspaceClass(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir(), ID: "default"})
	if err != nil {
		t.Fatalf("OpenWorkspace() error = %v", err)
	}
	runner := NewRunner(ws, RunnerOptions{Version: "test"})

	result, err := runner.Run(t.Context(), RunRequest{
		AnonymousBody: "Account account = AccountPlayground.makeAccount('Twin Lakes Supply'); System.debug(account.Name);",
		Mode:          RunModeScratch,
		LimitMode:     "permissive",
		UseCache:      false,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != RunStatusPass {
		t.Fatalf("status = %q diagnostics=%#v", result.Status, result.Diagnostics)
	}
	if len(result.Logs) != 1 || result.Logs[0] != "Twin Lakes Supply" {
		t.Fatalf("logs = %#v", result.Logs)
	}
	if len(result.OrgDiff) == 0 || result.OrgDiff[0].Object != "Account" || result.OrgDiff[0].Inserted != 1 {
		t.Fatalf("org diff = %#v", result.OrgDiff)
	}
}

func TestRunnerReturnsDBBootstrapError(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir(), ID: "default"})
	if err != nil {
		t.Fatalf("OpenWorkspace() error = %v", err)
	}
	dbPath := filepath.Join(t.TempDir(), "not-a-db")
	if err := os.Mkdir(dbPath, 0o755); err != nil {
		t.Fatal(err)
	}
	runner := NewRunner(ws, RunnerOptions{Version: "test", DBPath: dbPath})

	_, err = runner.Run(t.Context(), RunRequest{
		AnonymousBody: "System.debug('probe');",
		Mode:          RunModeScratch,
		LimitMode:     "permissive",
		UseCache:      false,
	})
	if err == nil {
		t.Fatalf("Run() error = nil, want DB bootstrap error")
	}
	if !strings.Contains(err.Error(), "not-a-db") {
		t.Fatalf("Run() error = %v, want DB path detail", err)
	}
}

func TestRunnerResetReturnsDBSaveError(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir(), ID: "default"})
	if err != nil {
		t.Fatalf("OpenWorkspace() error = %v", err)
	}
	dbPath := filepath.Join(t.TempDir(), "not-a-db")
	if err := os.Mkdir(dbPath, 0o755); err != nil {
		t.Fatal(err)
	}
	runner := NewRunner(ws, RunnerOptions{
		Version: "test",
		Org:     &vmOrgStore{org: storage.NewOrgState(), db: dbPath},
	})

	err = runner.Reset()
	if err == nil {
		t.Fatalf("Reset() error = nil, want DB save error")
	}
	if !strings.Contains(err.Error(), "not-a-db") {
		t.Fatalf("Reset() error = %v, want DB path detail", err)
	}
}

func TestRunnerBaselineOrgSupportsAccountCompoundAddress(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir(), ID: "default"})
	if err != nil {
		t.Fatalf("OpenWorkspace() error = %v", err)
	}
	runner := NewRunner(ws, RunnerOptions{Version: "test"})

	result, err := runner.Run(t.Context(), RunRequest{
		AnonymousBody: `
Account account = new Account(Name = 'Acme', BillingStreet = '12 Lake Road', BillingCity = 'Port Alsworth');
insert account;
Account queried = [SELECT Id, BillingAddress FROM Account WHERE Id = :account.Id LIMIT 1];
System.debug(queried.BillingAddress.street + ' / ' + queried.BillingAddress.city);
`,
		Mode:      RunModeScratch,
		LimitMode: "permissive",
		UseCache:  false,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != RunStatusPass {
		t.Fatalf("status = %q diagnostics=%#v error=%s", result.Status, result.Diagnostics, result.ErrorMessage)
	}
	if len(result.Logs) != 1 || result.Logs[0] != "12 Lake Road / Port Alsworth" {
		t.Fatalf("logs = %#v", result.Logs)
	}
}

func TestRunnerTreatsNamespacedProjectAsLocalSource(t *testing.T) {
	projectRoot := t.TempDir()
	writePlaygroundTestFile(t, filepath.Join(projectRoot, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"name":"local-project","namespace":"samplepkg","sourceApiVersion":"65.0"}`)
	writePlaygroundTestFile(t, filepath.Join(projectRoot, "force-app/main/default/classes/NextGenSettingService.cls"), `public class NextGenSettingService {
  public static String activateNextGenSetting() {
    return 'activated';
  }
}
`)
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir(), ProjectRoot: projectRoot, ID: "default"})
	if err != nil {
		t.Fatalf("OpenWorkspace() error = %v", err)
	}
	runner := NewRunner(ws, RunnerOptions{Version: "test"})

	result, err := runner.Run(t.Context(), RunRequest{
		AnonymousBody: "System.debug(NextGenSettingService.activateNextGenSetting());",
		Mode:          RunModeScratch,
		LimitMode:     "permissive",
		UseCache:      false,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != RunStatusPass {
		t.Fatalf("status = %q diagnostics=%#v error=%s", result.Status, result.Diagnostics, result.ErrorMessage)
	}
	if len(result.Logs) != 1 || result.Logs[0] != "activated" {
		t.Fatalf("logs = %#v", result.Logs)
	}
}

func TestRunnerLoadsProjectReferenceCustomObjectSchema(t *testing.T) {
	projectRoot := t.TempDir()
	writePlaygroundTestFile(t, filepath.Join(projectRoot, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"name":"local-project","namespace":"samplepkg","sourceApiVersion":"65.0"}`)
	writePlaygroundTestFile(t, filepath.Join(projectRoot, "force-app/main/default/classes/NextGenSettingService.cls"), `public class NextGenSettingService {
  public static void activateNextGenSetting() {
    SampleProtectedListSetting__c setting = new SampleProtectedListSetting__c();
    setting.Name = 'Default';
    upsert setting;
    System.debug('activated');
  }
}
`)
	writePlaygroundTestFile(t, filepath.Join(projectRoot, "force-app/main/default/objects/SampleProtectedListSetting__c/SampleProtectedListSetting__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata">
  <label>Sample Protected List Setting</label>
  <pluralLabel>Sample Protected List Settings</pluralLabel>
  <customSettingsType>List</customSettingsType>
  <visibility>Protected</visibility>
</CustomObject>`)
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir(), ID: "default"})
	if err != nil {
		t.Fatalf("OpenWorkspace() error = %v", err)
	}
	if _, err := ws.LoadProjectReference(ProjectReference{ID: "local", Path: projectRoot}); err != nil {
		t.Fatalf("LoadProjectReference() error = %v", err)
	}
	runner := NewRunner(ws, RunnerOptions{Version: "test"})

	result, err := runner.Run(t.Context(), RunRequest{
		AnonymousBody: "NextGenSettingService.activateNextGenSetting();",
		Mode:          RunModeScratch,
		LimitMode:     "permissive",
		UseCache:      false,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != RunStatusPass {
		t.Fatalf("status = %q diagnostics=%#v error=%s", result.Status, result.Diagnostics, result.ErrorMessage)
	}
	if len(result.Logs) != 1 || result.Logs[0] != "activated" {
		t.Fatalf("logs = %#v", result.Logs)
	}
	if len(result.OrgDiff) == 0 || result.OrgDiff[0].Object != "SampleProtectedListSetting__c" || result.OrgDiff[0].Inserted != 1 {
		t.Fatalf("org diff = %#v", result.OrgDiff)
	}
}

func TestRunnerUsesCacheForRepeatedRun(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir(), ID: "default"})
	if err != nil {
		t.Fatalf("OpenWorkspace() error = %v", err)
	}
	runner := NewRunner(ws, RunnerOptions{Version: "test"})
	req := RunRequest{AnonymousBody: "System.debug('cached');", Mode: RunModeScratch, LimitMode: "permissive", UseCache: true}

	first, err := runner.Run(t.Context(), req)
	if err != nil {
		t.Fatalf("first Run() error = %v", err)
	}
	second, err := runner.Run(t.Context(), req)
	if err != nil {
		t.Fatalf("second Run() error = %v", err)
	}
	if first.CacheHit {
		t.Fatalf("first run cacheHit = true")
	}
	if !second.CacheHit {
		t.Fatalf("second run cacheHit = false")
	}
}

func TestRunnerRecompilesWorkspaceClassAfterSourceChange(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir(), ID: "default"})
	if err != nil {
		t.Fatalf("OpenWorkspace() error = %v", err)
	}
	meta, err := ws.Metadata()
	if err != nil {
		t.Fatalf("Metadata() error = %v", err)
	}
	versions := make(map[string]int)
	for _, file := range meta.Files {
		versions[file.Path] = file.Version
	}
	path := "force-app/main/default/classes/AccountPlayground.cls"
	first := `public class AccountPlayground {
  public static String marker() {
    return 'first';
  }
}
`
	save, err := ws.SaveFile(FileSaveRequest{Path: path, Content: first, Version: versions[path]})
	if err != nil {
		t.Fatalf("first SaveFile() error = %v", err)
	}
	runner := NewRunner(ws, RunnerOptions{Version: "test"})
	req := RunRequest{AnonymousBody: "System.debug(AccountPlayground.marker());", Mode: RunModeScratch, LimitMode: "permissive", UseCache: false}

	firstRun, err := runner.Run(t.Context(), req)
	if err != nil {
		t.Fatalf("first Run() error = %v", err)
	}
	if len(firstRun.Logs) != 1 || firstRun.Logs[0] != "first" {
		t.Fatalf("first logs = %#v", firstRun.Logs)
	}

	second := `public class AccountPlayground {
  public static String marker() {
    return 'second';
  }
}
`
	if _, err := ws.SaveFile(FileSaveRequest{Path: path, Content: second, Version: save.File.Version}); err != nil {
		t.Fatalf("second SaveFile() error = %v", err)
	}
	secondRun, err := runner.Run(t.Context(), req)
	if err != nil {
		t.Fatalf("second Run() error = %v", err)
	}
	if len(secondRun.Logs) != 1 || secondRun.Logs[0] != "second" {
		t.Fatalf("second logs = %#v", secondRun.Logs)
	}
}

func TestRunnerCachesProjectRuntimeBetweenAnonymousRuns(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir(), ID: "default"})
	if err != nil {
		t.Fatalf("OpenWorkspace() error = %v", err)
	}
	runner := NewRunner(ws, RunnerOptions{Version: "test"})
	loads := 0
	runner.loadWorkspaceIndex = func(root string) (typesys.Index, []diagnostic.Diagnostic, error) {
		loads++
		return loadWorkspaceIndex(root)
	}

	for _, body := range []string{"System.debug('one');", "System.debug('two');"} {
		result, err := runner.Run(t.Context(), RunRequest{
			AnonymousBody: body,
			Mode:          RunModeScratch,
			LimitMode:     "permissive",
			UseCache:      false,
		})
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if result.Status != RunStatusPass {
			t.Fatalf("status = %q diagnostics=%#v error=%s", result.Status, result.Diagnostics, result.ErrorMessage)
		}
	}
	if loads != 1 {
		t.Fatalf("workspace loads = %d, want 1", loads)
	}
}

func TestRunnerKeepsProjectRuntimeAfterAnonymousFileSave(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir(), ID: "default"})
	if err != nil {
		t.Fatalf("OpenWorkspace() error = %v", err)
	}
	meta, err := ws.Metadata()
	if err != nil {
		t.Fatalf("Metadata() error = %v", err)
	}
	versions := make(map[string]int)
	for _, file := range meta.Files {
		versions[file.Path] = file.Version
	}
	runner := NewRunner(ws, RunnerOptions{Version: "test"})
	loads := 0
	runner.loadWorkspaceIndex = func(root string) (typesys.Index, []diagnostic.Diagnostic, error) {
		loads++
		return loadWorkspaceIndex(root)
	}

	for _, body := range []string{"System.debug('before');", "System.debug('after');"} {
		result, err := runner.Run(t.Context(), RunRequest{
			AnonymousBody: body,
			Mode:          RunModeScratch,
			LimitMode:     "permissive",
			UseCache:      false,
		})
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if result.Status != RunStatusPass {
			t.Fatalf("status = %q diagnostics=%#v error=%s", result.Status, result.Diagnostics, result.ErrorMessage)
		}
		if body == "System.debug('before');" {
			save, err := ws.SaveFile(FileSaveRequest{Path: "anonymous.apex", Content: "System.debug('saved');\n", Version: versions["anonymous.apex"]})
			if err != nil {
				t.Fatalf("SaveFile() error = %v", err)
			}
			versions["anonymous.apex"] = save.File.Version
		}
	}
	if loads != 1 {
		t.Fatalf("workspace loads = %d, want 1", loads)
	}
}

func TestExampleProjectExecutionPlanCoversEveryProjectOnce(t *testing.T) {
	plan := exampleProjectExecutionPlan(t)
	examples := ListExampleProjects()
	if len(plan) != 13 || len(examples) != 13 {
		t.Fatalf("execution plan/examples = %d/%d, want 13/13", len(plan), len(examples))
	}
	planSeen := make(map[string]int, len(plan))
	for _, testCase := range plan {
		planSeen[testCase.example.ID]++
	}
	for _, example := range examples {
		if planSeen[example.ID] != 1 {
			t.Fatalf("execution plan count for %q = %d, want 1", example.ID, planSeen[example.ID])
		}
	}
	wantExpectedLogs := map[string]bool{
		"bulk-trigger-rollup":        true,
		"map-selector-drill":         true,
		"contact-relationship-drill": true,
		"limit-counter-drill":        true,
		"deal-desk-discount-guard":   true,
		"renewal-health-scorecard":   true,
		"org-diff-review-loop":       true,
	}
	for _, testCase := range plan {
		if testCase.expectedLog == "" {
			continue
		}
		if !wantExpectedLogs[testCase.example.ID] {
			t.Fatalf("unexpected expected-log example %q", testCase.example.ID)
		}
		delete(wantExpectedLogs, testCase.example.ID)
	}
	if len(wantExpectedLogs) != 0 {
		t.Fatalf("examples missing expected-log checks: %#v", wantExpectedLogs)
	}
	want := map[string]bool{
		"refinement-service": true, "trigger-contact-task": true, "collection-selector": true,
		"persist-mode-ledger": true, "bulk-trigger-rollup": true, "map-selector-drill": true,
		"contact-relationship-drill": true, "governor-limits-strict": true, "org-diff-dml": true,
		"deal-desk-discount-guard": true, "renewal-health-scorecard": true,
		"org-diff-review-loop": true, "limit-counter-drill": true,
	}
	seen := make(map[string]int, len(plan))
	wantGroupSizes := []int{4, 3, 3, 3}
	for group := 0; group < 4; group++ {
		selected := exampleProjectExecutionGroup(plan, group)
		if len(selected) == 0 {
			t.Fatalf("group %d is empty", group)
		}
		if len(selected) != wantGroupSizes[group] {
			t.Fatalf("group %d has %d examples, want %d", group, len(selected), wantGroupSizes[group])
		}
		for _, testCase := range selected {
			seen[testCase.example.ID]++
		}
	}
	if len(seen) != 13 {
		t.Fatalf("group union has %d IDs, want 13: %#v", len(seen), seen)
	}
	for _, testCase := range plan {
		if seen[testCase.example.ID] != 1 {
			t.Fatalf("group union count for %q = %d, want 1", testCase.example.ID, seen[testCase.example.ID])
		}
		if !want[testCase.example.ID] {
			t.Fatalf("group union has unexpected ID %q", testCase.example.ID)
		}
		delete(want, testCase.example.ID)
	}
	if len(want) != 0 {
		t.Fatalf("group union is missing exact IDs: %#v", want)
	}
}

type exampleProjectExecutionTestCase struct {
	example     ExampleProject
	expectedLog string
}

func exampleProjectExecutionPlan(t *testing.T) []exampleProjectExecutionTestCase {
	t.Helper()
	expectedLogs := map[string]string{
		"bulk-trigger-rollup":        "AUTO-3",
		"map-selector-drill":         "Energy => 2",
		"contact-relationship-drill": "contacts: 3",
		"limit-counter-drill":        "dml rows:",
		"deal-desk-discount-guard":   "top bucket: strategic",
		"renewal-health-scorecard":   "health score: 85",
		"org-diff-review-loop":       "decision: approved",
	}
	examples := ListExampleProjects()
	plan := make([]exampleProjectExecutionTestCase, 0, len(examples))
	seen := make(map[string]bool, len(examples))
	for _, example := range examples {
		if seen[example.ID] {
			t.Fatalf("duplicate example id %q", example.ID)
		}
		seen[example.ID] = true
		plan = append(plan, exampleProjectExecutionTestCase{
			example:     example,
			expectedLog: expectedLogs[example.ID],
		})
	}
	for id := range expectedLogs {
		if !seen[id] {
			t.Fatalf("expected-log example %q is not listed", id)
		}
	}
	return plan
}

func exampleProjectExecutionGroup(plan []exampleProjectExecutionTestCase, group int) []exampleProjectExecutionTestCase {
	selected := make([]exampleProjectExecutionTestCase, 0, (len(plan)+3)/4)
	for index, testCase := range plan {
		if index%4 == group {
			selected = append(selected, testCase)
		}
	}
	return selected
}

func TestExampleProjectsRunAnonymousGroupOne(t *testing.T) {
	runExampleProjectExecutionGroup(t, 0)
}

func TestExampleProjectsRunAnonymousGroupTwo(t *testing.T) {
	runExampleProjectExecutionGroup(t, 1)
}

func TestExampleProjectsRunAnonymousGroupThree(t *testing.T) {
	runExampleProjectExecutionGroup(t, 2)
}

func TestExampleProjectsRunAnonymousGroupFour(t *testing.T) {
	runExampleProjectExecutionGroup(t, 3)
}

func runExampleProjectExecutionGroup(t *testing.T, group int) {
	t.Helper()
	plan := exampleProjectExecutionGroup(exampleProjectExecutionPlan(t), group)
	executed := make(map[string]int, len(plan))
	for _, testCase := range plan {
		t.Run(testCase.example.ID, func(t *testing.T) {
			executed[testCase.example.ID]++
			ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir(), ID: "default"})
			if err != nil {
				t.Fatalf("OpenWorkspace() error = %v", err)
			}
			meta, err := ws.LoadExample(testCase.example.ID)
			if err != nil {
				t.Fatalf("LoadExample() error = %v", err)
			}
			if testCase.example.ID == "deal-desk-discount-guard" || testCase.example.ID == "renewal-health-scorecard" || testCase.example.ID == "org-diff-review-loop" {
				sourceFiles := 0
				hasAnonymous := false
				for _, file := range meta.Files {
					switch file.Kind {
					case "class", "trigger":
						sourceFiles++
					case "anonymous":
						hasAnonymous = true
					}
				}
				if sourceFiles < 3 || !hasAnonymous {
					t.Fatalf("example %q files: source=%d anonymous=%t files=%#v", testCase.example.ID, sourceFiles, hasAnonymous, meta.Files)
				}
			}
			runner := NewRunner(ws, RunnerOptions{Version: "test"})
			result, err := runner.Run(t.Context(), RunRequest{
				AnonymousBody: meta.AnonymousBody,
				Mode:          RunModeScratch,
				LimitMode:     "permissive",
				UseCache:      false,
			})
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if result.Status != RunStatusPass {
				t.Fatalf("status = %q diagnostics=%#v error=%s", result.Status, result.Diagnostics, result.ErrorMessage)
			}
			if len(result.Logs) == 0 {
				t.Fatalf("logs = %#v", result.Logs)
			}
			if testCase.expectedLog != "" {
				joined := strings.Join(result.Logs, "\n")
				if !strings.Contains(joined, testCase.expectedLog) {
					t.Fatalf("logs missing %q: %#v", testCase.expectedLog, result.Logs)
				}
			}
		})
	}
	for _, testCase := range plan {
		if executed[testCase.example.ID] != 1 {
			t.Fatalf("execution count for %q = %d, want 1", testCase.example.ID, executed[testCase.example.ID])
		}
	}
}

func TestRunnerReportsCompileErrorWithoutCommit(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir(), ID: "default"})
	if err != nil {
		t.Fatalf("OpenWorkspace() error = %v", err)
	}
	runner := NewRunner(ws, RunnerOptions{Version: "test"})

	result, err := runner.Run(t.Context(), RunRequest{
		AnonymousBody: "Account a = ; insert a;",
		Mode:          RunModePersist,
		LimitMode:     "permissive",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != RunStatusCompileError {
		t.Fatalf("status = %q", result.Status)
	}
	if len(result.Diagnostics) == 0 || !strings.Contains(result.Diagnostics[0].Message, "unexpected") {
		t.Fatalf("diagnostics = %#v", result.Diagnostics)
	}
	latest := runner.Org()
	if len(latest.Objects["Account"].Records) != 0 {
		t.Fatalf("compile error committed records: %#v", latest.Objects["Account"].Records)
	}
}
