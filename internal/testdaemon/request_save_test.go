package testdaemon

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
)

func TestRequestSaveReplay(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	root, _ := editSnapshotProject(t)
	other := filepath.Join(root, "force-app/main/default/classes/Other.cls")
	writeFile(t, other, `public class Other { public static Integer value() { return 0; } }`)
	d, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	ready := make(chan struct{})
	close(ready)
	server := &Server{daemon: d, ready: true, warmDone: ready}
	request := NewRunRequestV1(apextest.Options{NoDiskCache: true, Parallelism: 1}, "", 0, 0, false)
	for i := range 6 {
		writeFile(t, other, fmt.Sprintf(`public class Other { public static Integer value() { return %d; } }`, i+1))
		response := exchangeProtocolWiringV1(t, server, RequestV1{Version: ProtocolVersionV1, Op: OpRun, Run: &request})
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
}

func TestRequestSaveRediscoversCases(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	root, classPath := editSnapshotProject(t)
	d, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, classPath, `@IsTest private class EditTest { @IsTest static void passes() {} @IsTest static void added() {} }`)
	request := NewRunRequestV1(apextest.Options{NoDiskCache: true, Parallelism: 1}, "", 1, 0, false)
	var events []apextest.TestProgress
	run, _, _, err := d.RunRequestV1(context.Background(), request, func(event apextest.TestProgress) {
		events = append(events, event)
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if summary := run.Summary(); summary.Total != 2 || summary.Passed != 2 {
		t.Fatalf("summary=%#v problems=%s", summary, runProblems(run))
	}
	for _, event := range events {
		if event.Event == "test_done" && event.Status == "compile_error" {
			t.Fatalf("discarded attempt emitted a compile error: %#v", events)
		}
	}
}
