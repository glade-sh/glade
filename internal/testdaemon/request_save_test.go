package testdaemon

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
)

func TestRequestSaveReplay(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	root, _ := editSnapshotProject(t)
	other := filepath.Join(root, "force-app/main/default/classes/Other.cls")
	writeFile(t, other, `public class Other { public static Integer value() { return 0; } }`)
	server, err := NewServer(ServerConfig{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	served := make(chan error, 1)
	go func() { served <- server.ListenAndServe(ctx, io.Discard) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-served:
			if err != nil {
				t.Errorf("serve: %v", err)
			}
		case <-time.After(2 * time.Minute):
			t.Error("server did not stop")
		}
	})
	waitForServerCondition(t, func() bool {
		_, err := PingV1(context.Background(), server.socket)
		return err == nil
	}, "server did not start")
	request := NewRunRequestV1(apextest.Options{NoDiskCache: true, Parallelism: 1}, "", 0, 0, false)
	for i := range 6 {
		writeFile(t, other, fmt.Sprintf(`public class Other { public static Integer value() { return %d; } }`, i+1))
		response, err := RunV1(context.Background(), server.socket, request)
		if err != nil {
			t.Fatal(err)
		}
		if !response.OK || response.Run == nil {
			t.Fatalf("request %d: %#v", i, response)
		}
		if summary := response.Run.Summary(); summary.Total != 1 || summary.Passed != 1 {
			t.Fatalf("request %d: summary=%#v problems=%s", i, summary, runProblems(*response.Run))
		}
	}
}

func TestRequestRealCompileError(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	root, classPath := editSnapshotProject(t)
	writeFile(t, classPath, `@IsTest private class EditTest { @IsTest static void passes() { Integer x = missing; } }`)
	d, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	ready := make(chan struct{})
	close(ready)
	server := &Server{daemon: d, ready: true, warmDone: ready}
	request := NewRunRequestV1(apextest.Options{NoDiskCache: true, Parallelism: 1}, "", 0, 0, false)
	response := exchangeProtocolWiringV1(t, server, RequestV1{Version: ProtocolVersionV1, Op: OpRun, Run: &request})
	if !response.OK || response.Run == nil || response.Run.Summary().CompileErrors == 0 || !strings.Contains(runProblems(*response.Run), "missing") {
		t.Fatalf("real compile error lost: %#v", response)
	}
	encoded, err := json.Marshal(response.Run)
	if err != nil {
		t.Fatal(err)
	}
	canonical := regexp.MustCompile(`"durationMs":[0-9]+`).ReplaceAllString(string(encoded), `"durationMs":0`)
	t.Logf("neighbor report: %s", strings.ReplaceAll(canonical, root, "<root>"))
}

func TestRequestSaveRediscoversCases(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	root, classPath := editSnapshotProject(t)
	d, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, classPath, `@IsTest private class EditTest { @IsTest static void passes() {} @IsTest static void added() {} }`)
	request := NewRunRequestV1(apextest.Options{NoDiskCache: true, Parallelism: 2, ParallelMethods: true}, "", 1, 0, false)
	events := make(chan apextest.TestProgress, 128)
	run, _, _, err := d.RunRequestV1(context.Background(), request, func(event apextest.TestProgress) {
		events <- event
	}, nil)
	close(events)
	if err != nil {
		t.Fatal(err)
	}
	if summary := run.Summary(); summary.Total != 2 || summary.Passed != 2 {
		t.Fatalf("summary=%#v problems=%s", summary, runProblems(run))
	}
	for event := range events {
		if event.Event == "test_done" && event.Status == "compile_error" {
			t.Fatalf("discarded attempt emitted a compile error: %#v", event)
		}
	}
}

func TestRequestSaveKeepsRealCompileError(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	root, classPath := editSnapshotProject(t)
	writeFile(t, classPath, `@IsTest private class EditTest { @IsTest static void passes() { Integer x = missing; } }`)
	other := filepath.Join(root, "force-app/main/default/classes/Other.cls")
	writeFile(t, other, `public class Other {}`)
	d, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, other, `public class Other { /* saved */ }`)
	request := NewRunRequestV1(apextest.Options{NoDiskCache: true, Parallelism: 1}, "", 0, 0, false)
	run, _, _, err := d.RunRequestV1(context.Background(), request, nil, nil)
	if err != nil || run.Summary().CompileErrors == 0 || !strings.Contains(runProblems(run), "missing") || strings.Contains(runProblems(run), "source snapshot mismatch") {
		t.Fatalf("real error after save: summary=%#v error=%v problems=%s", run.Summary(), err, runProblems(run))
	}
}

func TestRequestFailedRefreshReportsOriginalErrorAndProgress(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	root, classPath := editSnapshotProject(t)
	d, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, classPath, `@IsTest private class EditTest { /* saved */ @IsTest static void passes() {} }`)
	d.loadProjectFn = func(string) (project.Project, error) { return project.Project{}, fmt.Errorf("reload failed") }
	request := NewRunRequestV1(apextest.Options{NoDiskCache: true, Parallelism: 1}, "", 0, 0, false)
	var events []apextest.TestProgress
	run, _, _, err := d.RunRequestV1(context.Background(), request, func(event apextest.TestProgress) {
		events = append(events, event)
	}, nil)
	if err != nil || run.Summary().CompileErrors != 1 || !strings.Contains(runProblems(run), "source snapshot mismatch") {
		t.Fatalf("failed refresh: summary=%#v error=%v problems=%s", run.Summary(), err, runProblems(run))
	}
	for _, event := range events {
		if event.Event == "test_done" && event.Status == "compile_error" {
			return
		}
	}
	t.Fatalf("failed refresh lost compile error progress: %#v", events)
}

func TestRequestDoesNotRepeatCompletedTestsAfterSave(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	root, _ := editSnapshotProject(t)
	classes := filepath.Join(root, "force-app/main/default/classes")
	writeFile(t, filepath.Join(classes, "Broken.cls"), `public class Broken { public static Integer value() { return missing; } }`)
	writeFile(t, filepath.Join(classes, "BrokenTest.cls"), `@IsTest private class BrokenTest { @IsTest static void failsCompile() { Broken.value(); } }`)
	other := filepath.Join(classes, "Other.cls")
	writeFile(t, other, `public class Other {}`)
	d, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	request := NewRunRequestV1(apextest.Options{NoDiskCache: true, Parallelism: 1}, "", 0, 0, false)
	starts := 0
	run, _, _, err := d.RunRequestV1(context.Background(), request, func(event apextest.TestProgress) {
		if event.Event == "test_start" {
			starts++
			if starts == 1 {
				writeFile(t, other, `public class Other { /* saved while a test ran */ }`)
			}
		}
	}, nil)
	if err != nil || run.Summary().Passed != 1 || run.Summary().CompileErrors != 1 || starts != 1 {
		t.Fatalf("mixed result: summary=%#v error=%v starts=%d problems=%s", run.Summary(), err, starts, runProblems(run))
	}
}

func TestRequestCompileProgressAfterSave(t *testing.T) {
	for _, save := range []bool{false, true} {
		name := "unchanged"
		if save {
			name = "save during compile"
		}
		t.Run(name, func(t *testing.T) {
			t.Cleanup(apextest.InvalidateRuntimeCaches)
			root, classPath := editSnapshotProject(t)
			d, err := New(root)
			if err != nil {
				t.Fatal(err)
			}
			request := NewRunRequestV1(apextest.Options{NoDiskCache: true, Parallelism: 1}, "", 0, 0, false)
			var events []string
			changed := false
			run, _, _, err := d.RunRequestV1(context.Background(), request, func(event apextest.TestProgress) {
				if event.Event == "compile_start" || event.Event == "compile_done" {
					events = append(events, event.Event+":"+event.Status)
				}
				if save && !changed && event.Event == "compile_start" {
					changed = true
					writeFile(t, classPath, `@IsTest private class EditTest { @IsTest static void passes() {} @IsTest static void added() {} }`)
				}
			}, nil)
			if err != nil {
				t.Fatal(err)
			}
			wantTotal := 1
			if save {
				wantTotal = 2
			}
			if summary := run.Summary(); summary.Total != wantTotal || summary.Passed != wantTotal {
				t.Fatalf("summary=%#v problems=%s", summary, runProblems(run))
			}
			if got := strings.Join(events, ","); got != "compile_start:,compile_done:pass" {
				t.Fatalf("compile events = %s", got)
			}
		})
	}
}

func TestRequestCompileProgressWhenRetrySelectorDisappears(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	root, classPath := editSnapshotProject(t)
	d, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	request := NewRunRequestV1(apextest.Options{NoDiskCache: true, Parallelism: 1, SelectedClasses: []string{"EditTest"}, SelectedMethod: "passes"}, "", 0, 0, false)
	var events []string
	changed := false
	run, _, _, err := d.RunRequestV1(context.Background(), request, func(event apextest.TestProgress) {
		if event.Event == "compile_start" || event.Event == "compile_done" {
			events = append(events, event.Event+":"+event.Status)
		}
		if !changed && event.Event == "compile_start" {
			changed = true
			writeFile(t, classPath, `@IsTest private class EditTest { @IsTest static void other() {} }`)
		}
	}, nil)
	if err != nil || run.Summary().Errors != 1 || !strings.Contains(runProblems(run), "no test method matched") {
		t.Fatalf("selector result: summary=%#v error=%v problems=%s", run.Summary(), err, runProblems(run))
	}
	if got := strings.Join(events, ","); got != "compile_start:,compile_done:fail" {
		t.Fatalf("compile events = %s", got)
	}
}
