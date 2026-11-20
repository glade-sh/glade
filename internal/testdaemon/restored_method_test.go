package testdaemon

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/watch"
)

func TestWatchedRestoredMethodMatchesFreshDaemon(t *testing.T) {
	for _, backend := range []watch.Backend{watch.BackendPoll, watch.BackendNative} {
		t.Run(string(backend), func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "force-app/main/default/classes/ExampleTest.cls")
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
			bodies := []string{"@IsTest private class ExampleTest { @IsTest static void first() {} }", "@IsTest private class ExampleTest { @IsTest static void other() {} }"}
			writeFile(t, path, bodies[0])
			stamp := time.Unix(1600000000, 0)
			if err := os.Chtimes(path, stamp, stamp); err != nil {
				t.Fatal(err)
			}
			daemon, err := New(root)
			if err != nil {
				t.Fatal(err)
			}
			scope := daemon.WatchScopeSnapshot(root)
			baseline, err := watch.CaptureScope(scope)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			watcher, _, err := watch.NewBackendWatcher(ctx, watch.Config{Root: root, Scope: scope, Backend: backend, Debounce: 20 * time.Millisecond}, baseline)
			if err != nil {
				t.Fatal(err)
			}
			defer watcher.Close()
			for i, method := range []string{"other", "first"} {
				writeFile(t, path, bodies[(i+1)%2])
				if err := os.Chtimes(path, stamp, stamp); err != nil {
					t.Fatal(err)
				}
				select {
				case changes := <-watcher.Changes():
					if err := daemon.UpdateChanges(changes); err != nil {
						t.Fatal(err)
					}
				case err := <-watcher.Errors():
					t.Fatalf("watch: %v", err)
				case <-time.After(2 * time.Second):
					t.Fatal("watcher missed restored content")
				}
				fresh, err := New(root)
				if err != nil {
					t.Fatal(err)
				}
				got := apextest.Discover(daemon.IndexSnapshot(), apextest.Options{})
				want := apextest.Discover(fresh.IndexSnapshot(), apextest.Options{})
				if !reflect.DeepEqual(got, want) || len(got) != 1 || got[0].MethodName != method {
					t.Fatalf("discovered = %#v, fresh = %#v", got, want)
				}
				for _, selected := range []string{"first", "other"} {
					request := RunRequestV1{Parallelism: 1, NoDiskCache: true, SelectedClasses: []string{"ExampleTest"}, SelectedMethod: selected}
					planRequest := request
					planRequest.ShardCount = 1
					planRequest.ReturnClassShards = true
					gotPlanRun, gotSelection, gotPlan, err := daemon.runRequestV1(ctx, planRequest)
					if err != nil {
						t.Fatal(err)
					}
					wantPlanRun, wantSelection, wantPlan, err := fresh.runRequestV1(ctx, planRequest)
					if err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(gotPlanRun, wantPlanRun) || !reflect.DeepEqual(gotSelection, wantSelection) || !reflect.DeepEqual(gotPlan, wantPlan) {
						t.Fatalf("plan for %s differs from fresh: run=%#v selection=%#v plan=%#v; want %#v %#v %#v", selected, gotPlanRun, gotSelection, gotPlan, wantPlanRun, wantSelection, wantPlan)
					}
					gotRun, _, _, err := daemon.runRequestV1(ctx, request)
					if err != nil {
						t.Fatal(err)
					}
					wantRun, _, _, err := fresh.runRequestV1(ctx, request)
					if err != nil {
						t.Fatal(err)
					}
					if selected == method {
						if summary := gotRun.Summary(); summary.Total != 1 || summary.Passed != 1 {
							t.Fatalf("current method %s did not pass: %#v", selected, gotRun)
						}
					} else {
						data, err := json.Marshal(gotRun)
						if err != nil {
							t.Fatal(err)
						}
						if !strings.Contains(string(data), "no test method matched") {
							t.Fatalf("removed method %s: %s", selected, data)
						}
					}
					if !reflect.DeepEqual(canonicalDaemonRun(gotRun), canonicalDaemonRun(wantRun)) {
						t.Fatalf("selection %s: got %#v, fresh %#v", selected, gotRun, wantRun)
					}
				}
			}
		})
	}
}
