package gladecli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/testdaemon"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/watch"
)

func TestSupplementalSourceWatchLocalAndDaemonRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, mode := range []string{"local", "daemon"} {
			for _, backend := range []watch.Backend{watch.BackendPoll, watch.BackendNative} {
				t.Run(api+"/"+mode+"/"+string(backend), func(t *testing.T) {
					workspace := t.TempDir()
					root := filepath.Join(workspace, "project")
					extra := filepath.Join(workspace, "supplemental")
					service := filepath.Join(extra, "classes/SupplementalService.cls")
					network := filepath.Join(extra, "networks/Synthetic Portal.network-meta.xml")
					writeTestFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
					writeTestFile(t, filepath.Join(root, "glade.yml"), "org:\n  features: [Communities]\nproject:\n  extraMetadataDirs: [../supplemental]\n")
					writeTestFile(t, service, "public class SupplementalService { public static Integer value() { return 1; } }")
					writeTestFile(t, network, `<Network><status>UnderConstruction</status><urlPathPrefix>synthetic</urlPathPrefix></Network>`)
					writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/ServiceWatchTest.cls"), "@IsTest private class ServiceWatchTest { @IsTest static void edited() { System.assertEquals(2, SupplementalService.value()); } }")
					// The enabled-empty expectation is captured in network_configuration/org.tsv,
					// enabled/API 62 and 67/anonymous and istest/count = 0.
					writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/NetworkWatchTest.cls"), "@IsTest private class NetworkWatchTest { @IsTest static void removed() { System.assertEquals(0, Database.countQuery('SELECT COUNT() FROM Network')); } }")
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					cfg := watch.Config{Root: root, Backend: backend, Debounce: 10 * time.Millisecond}
					opts := apextest.Options{NoDiskCache: true}
					var watcher watch.BackendWatcher
					var apply func([]watch.Change) testreport.Run
					if mode == "local" {
						replacement, err := prepareInitialLocalWatchReplacement(ctx, root, cfg, backend, watch.CaptureScope)
						if err != nil {
							t.Fatal(err)
						}
						watcher = replacement.watcher
						apply = func(changes []watch.Change) testreport.Run {
							update, err := tryUpdateWatchIndexState(root, replacement.cfg.Scope, replacement.index, replacement.graph, changes)
							if err != nil {
								t.Fatal(err)
							}
							if update.reusable {
								replacement.index, replacement.graph = update.index, update.graph
							} else {
								next, err := prepareLocalWatchReplacement(ctx, root, replacement.cfg, backend)
								if err != nil {
									t.Fatal(err)
								}
								_ = watcher.Close()
								watcher, replacement = next.watcher, next
							}
							return runSelectedTests(replacement.index, opts, watch.SelectAffectedTestsWithRefGraph(replacement.index, changes, replacement.graph))
						}
					} else {
						daemon, err := testdaemon.New(root)
						if err != nil {
							t.Fatal(err)
						}
						watcher, cfg, err = prepareInitialDaemonWatch(ctx, daemon, cfg, backend, watch.CaptureScope)
						if err != nil {
							t.Fatal(err)
						}
						apply = func(changes []watch.Change) testreport.Run {
							exact, err := daemon.TryUpdateChanges(ctx, changes, cfg.Scope)
							if err != nil {
								t.Fatal(err)
							}
							if !exact {
								next, nextCfg, err := prepareInitialDaemonWatch(ctx, daemon, cfg, backend, watch.CaptureScope)
								if err != nil {
									t.Fatal(err)
								}
								_ = watcher.Close()
								watcher, cfg = next, nextCfg
							}
							return daemon.RunSelectionContext(ctx, opts, daemon.SelectAffected(changes))
						}
					}
					defer func() { _ = watcher.Close() }()
					writeTestFile(t, service, "public class SupplementalService { public static Integer value() { return 2; } }")
					changes := waitForWatchChanges(t, watcher, service, watch.ChangeModified)
					if run := apply(changes); run.Summary().Passed != 1 || run.Summary().Failed != 0 {
						t.Fatalf("supplemental Apex watch run = %#v", run)
					}
					if err := os.Remove(network); err != nil {
						t.Fatal(err)
					}
					changes = waitForWatchChanges(t, watcher, network, watch.ChangeDeleted)
					if run := apply(changes); run.Summary().Passed != 2 || run.Summary().Failed != 0 {
						t.Fatalf("supplemental Network removal watch run = %#v", run)
					}
				})
			}
		}
	}
}
