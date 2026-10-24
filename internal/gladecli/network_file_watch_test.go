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

func TestNetworkFileProjectWatchLocalAndDaemonRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, mode := range []string{"local", "daemon"} {
			for _, backend := range []watch.Backend{watch.BackendPoll, watch.BackendNative} {
				for _, location := range []string{"package", "supplemental"} {
					t.Run(api+"/"+mode+"/"+string(backend)+"/"+location, func(t *testing.T) {
						workspace := t.TempDir()
						root := filepath.Join(workspace, "project")
						extra := filepath.Join(workspace, "supplemental")
						writeTestFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
						writeTestFile(t, filepath.Join(root, "glade.yml"), "org:\n  features: [Communities]\nproject:\n  extraMetadataDirs: [../supplemental]\n")
						writeTestFile(t, filepath.Join(extra, "README.md"), "Supplemental metadata")
						metadataRoot := extra
						if location == "package" {
							metadataRoot = filepath.Join(root, "force-app/main/default")
						}
						network := filepath.Join(metadataRoot, "networks/Synthetic Portal.network")
						body := `<Network><status>UnderConstruction</status><urlPathPrefix>synthetic</urlPathPrefix></Network>`
						writeTestFile(t, network, body)
						// Both metadata states use captured enabled-empty/enabled-metadata counts:
						// network_configuration/org.tsv, API 62/67, anonymous and istest.
						// Snapshot inclusion must be checked before the asynchronous watcher starts.
						snapshot, err := watch.CaptureSnapshot(metadataRoot)
						if err != nil {
							t.Fatal(err)
						}
						if _, ok := snapshot.Files[network]; !ok {
							t.Fatal("project watch snapshot omitted supported .network file")
						}
						writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/NetworkFileWatchTest.cls"), `@IsTest private class NetworkFileWatchTest {
 @IsTest static void modified() { checkSite('Synthetic Portal'); }
 @IsTest static void renamed() { checkSite('Renamed Portal'); }
 @IsTest static void deleted() { System.assertEquals(0, Database.countQuery('SELECT COUNT() FROM Network')); }
 @IsTest static void added() { checkSite('Renamed Portal'); }
 private static void checkSite(String expectedName) {
  System.assertEquals(1, Database.countQuery('SELECT COUNT() FROM Network'));
  Network site = [SELECT Name, UrlPathPrefix FROM Network LIMIT 1];
  System.assertEquals(expectedName, site.Name);
  System.assertEquals('synthetic', site.UrlPathPrefix);
 }
}`)
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

						for _, operation := range []string{"modified", "renamed", "deleted", "added"} {
							t.Run(operation, func(t *testing.T) {
								eventPath, wantOp := network, watch.ChangeModified
								switch operation {
								case "modified":
									writeTestFile(t, network, body+"\n")
								case "renamed":
									next := filepath.Join(metadataRoot, "networks/Renamed Portal.network")
									if err := os.Rename(network, next); err != nil {
										t.Fatal(err)
									}
									network, eventPath, wantOp = next, next, watch.ChangeAdded
								case "deleted":
									if err := os.Remove(network); err != nil {
										t.Fatal(err)
									}
									wantOp = watch.ChangeDeleted
								case "added":
									writeTestFile(t, network, body)
									wantOp = watch.ChangeAdded
								}
								opts.SelectedMethod = operation
								changes := waitForWatchChanges(t, watcher, eventPath, wantOp)
								if run := apply(changes); run.Summary().Passed != 1 || run.Summary().Failed != 0 {
									t.Fatalf("Network file %s watch run = %#v", operation, run)
								}
							})
						}
					})
				}
			}
		}
	}
}
