package playground

import (
	"strings"
	"testing"
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

func TestExampleProjectsRunAnonymous(t *testing.T) {
	for _, example := range ListExampleProjects() {
		t.Run(example.ID, func(t *testing.T) {
			ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir(), ID: "default"})
			if err != nil {
				t.Fatalf("OpenWorkspace() error = %v", err)
			}
			meta, err := ws.LoadExample(example.ID)
			if err != nil {
				t.Fatalf("LoadExample() error = %v", err)
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
		})
	}
}

func TestComplexExampleProjectsRunAnonymous(t *testing.T) {
	want := map[string]string{
		"bulk-trigger-rollup":        "AUTO-3",
		"map-selector-drill":         "Energy => 2",
		"contact-relationship-drill": "contacts: 3",
		"limit-counter-drill":        "dml rows:",
	}
	for id, logText := range want {
		t.Run(id, func(t *testing.T) {
			ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir(), ID: "default"})
			if err != nil {
				t.Fatalf("OpenWorkspace() error = %v", err)
			}
			meta, err := ws.LoadExample(id)
			if err != nil {
				t.Fatalf("LoadExample() error = %v", err)
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
			joined := strings.Join(result.Logs, "\n")
			if !strings.Contains(joined, logText) {
				t.Fatalf("logs missing %q: %#v", logText, result.Logs)
			}
		})
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
