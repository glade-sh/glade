package testdaemon

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/watch"
)

func TestGracefulStopServerEmptyRunRoutes(t *testing.T) {
	for _, route := range []string{"before-admission", "queued", "warming", "execution"} {
		for _, cause := range []string{"cancel", "deadline"} {
			t.Run(route+"/"+cause, func(t *testing.T) {
				server := protocolWiringReadyServer(t)
				ctx, cancel := context.WithCancel(context.Background())
				stopErr := context.Canceled
				if cause == "deadline" {
					cancel()
					ctx, cancel = context.WithTimeout(context.Background(), 30*time.Millisecond)
					stopErr = context.DeadlineExceeded
				}
				defer cancel()
				switch route {
				case "before-admission":
					if cause == "cancel" {
						cancel()
					} else {
						<-ctx.Done()
					}
				case "queued":
					release, err := server.acquireRun(context.Background())
					if err != nil {
						t.Fatal(err)
					}
					defer release()
				case "warming":
					server.warmDone = make(chan struct{})
				}
				if cause == "cancel" && (route == "queued" || route == "warming") {
					timer := time.AfterFunc(20*time.Millisecond, cancel)
					defer timer.Stop()
				}
				server.runRequestV1 = func(context.Context, RunRequestV1) (testreport.Run, watch.TestSelection, *ClassShardPlanV1, error) {
					if route != "execution" {
						t.Error("stopped request entered execution")
					}
					return testreport.Run{}, watch.TestSelection{}, nil, stopErr
				}
				response := server.handleV1(ctx, RequestV1{Version: ProtocolVersionV1, Op: OpRun, Run: &RunRequestV1{Parallelism: 1}})
				assertQueuedStopResponse(t, response, stopErr)
			})
		}
	}
}

func TestGracefulStopServerEmptyRunRejectsNonStopErrors(t *testing.T) {
	for _, route := range []string{"validation", "busy", "warming", "execution"} {
		t.Run(route, func(t *testing.T) {
			server := protocolWiringReadyServer(t)
			request := RequestV1{Version: ProtocolVersionV1, Op: OpRun, Run: &RunRequestV1{Parallelism: 1}}
			failure := errors.New("synthetic run failure")
			switch route {
			case "validation":
				request.Run = nil
			case "busy":
				release, err := server.acquireRun(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				defer release()
				server.runAdmission <- struct{}{}
				defer func() { <-server.runAdmission }()
			case "warming":
				server.warmErr = failure
			}
			server.runRequestV1 = func(context.Context, RunRequestV1) (testreport.Run, watch.TestSelection, *ClassShardPlanV1, error) {
				return testreport.Run{}, watch.TestSelection{}, nil, failure
			}
			response := server.handleV1(context.Background(), request)
			if response.Op != OpError || response.OK || response.Error == "" || response.Run != nil {
				t.Fatalf("non-stop error gained a run: %#v", response)
			}
		})
	}
}

func TestGracefulStopQueuedSocketProjectRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, cause := range []string{"cancel", "deadline"} {
			t.Run(api+"/"+cause, func(t *testing.T) {
				t.Cleanup(apextest.InvalidateRuntimeCaches)
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
				classes := filepath.Join(root, "force-app/main/default/classes")
				writeFile(t, filepath.Join(classes, "QueuedTest.cls"), `@IsTest private class QueuedTest { @IsTest static void passes() { System.assertEquals(2, 1 + 1); } }`)
				writeFile(t, filepath.Join(classes, "QueuedTest.cls-meta.xml"), fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api))
				server, err := NewServer(ServerConfig{Root: root})
				if err != nil {
					t.Fatal(err)
				}
				serveCtx, stopServer := context.WithCancel(context.Background())
				served := make(chan error, 1)
				go func() { served <- server.ListenAndServe(serveCtx, io.Discard) }()
				t.Cleanup(func() {
					stopServer()
					if err := <-served; err != nil {
						t.Errorf("serve: %v", err)
					}
				})
				waitForServerCondition(t, func() bool {
					_, err := PingV1(context.Background(), server.socket)
					return err == nil
				}, "server did not start")
				// Hold the real execution slot so the wire request must queue.
				release, err := server.acquireRun(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if release != nil {
						release()
					}
				})
				ctx, cancel := context.WithCancel(context.Background())
				if cause == "deadline" {
					cancel()
					ctx, cancel = context.WithTimeout(context.Background(), 500*time.Millisecond)
				}
				defer cancel()
				type outcome struct {
					response ResponseV1
					err      error
				}
				finished := make(chan outcome, 1)
				go func() {
					response, err := RunV1(ctx, server.socket, RunRequestV1{Parallelism: 1})
					finished <- outcome{response, err}
				}()
				waitForServerCondition(t, func() bool { return len(server.runAdmission) == 2 }, "request never queued")
				if cause == "cancel" {
					cancel()
				}
				select {
				case result := <-finished:
					if result.err == nil || result.response.Error != context.Canceled.Error() {
						t.Errorf("queued client cancellation: response=%#v err=%v", result.response, result.err)
					}
					assertQueuedStopResponse(t, result.response, context.Canceled)
				case <-time.After(3 * time.Second):
					t.Fatal("queued stop did not return")
				}
				if len(server.runAdmission) != 1 || len(server.runExecution) != 1 {
					t.Errorf("queued stop leaked slots: admission=%d execution=%d", len(server.runAdmission), len(server.runExecution))
				}
				release()
				release = nil
				next, err := RunV1(context.Background(), server.socket, RunRequestV1{Parallelism: 1})
				if err != nil || !next.OK || next.Run == nil || next.Run.Summary().Passed != 1 || next.Run.Summary().Total != 1 {
					t.Fatalf("next project request did not execute: response=%#v err=%v", next, err)
				}
			})
		}
	}
}

func assertQueuedStopResponse(t *testing.T, response ResponseV1, stopErr error) {
	t.Helper()
	if response.Op != OpError || response.OK || response.Error != stopErr.Error() || response.Run == nil || response.Run.Summary().Total != 0 {
		t.Errorf("queued stop lost final empty run: %#v", response)
	}
}
