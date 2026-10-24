package gladecli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/testdaemon"
	"github.com/glade-sh/glade/internal/testreport"
)

func TestNetworkFileGitProjectCLIAliasAndDaemonRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, location := range []string{"package", "supplemental"} {
			for _, operation := range []string{"unstaged_edit", "staged_edit", "deleted", "renamed", "untracked"} {
				t.Run(api+"/"+location+"/"+operation, func(t *testing.T) {
					root := t.TempDir()
					writeTestFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
					writeTestFile(t, filepath.Join(root, "glade.yml"), "org:\n  features: [Communities]\nproject:\n  extraMetadataDirs: [supplemental]\n")
					writeTestFile(t, filepath.Join(root, "supplemental/README.md"), "Supplemental metadata")
					metadataRoot := filepath.Join(root, "force-app/main/default")
					if location == "supplemental" {
						metadataRoot = filepath.Join(root, "supplemental")
					}
					path := filepath.Join(metadataRoot, "networks/Synthetic Portal.network")
					body := `<Network><status>UnderConstruction</status><urlPathPrefix>synthetic</urlPathPrefix></Network>`
					writeTestFile(t, path, body)
					count, name := 1, "Synthetic Portal"
					switch operation {
					case "deleted":
						count = 0
					case "renamed":
						name = "Renamed Portal"
					case "untracked":
						count = 2
					}
					// Count and fields reuse network_configuration/org.tsv's enabled-empty
					// and enabled-metadata native rows at API 62/67, anonymous and istest.
					assertions := fmt.Sprintf("System.assertEquals(%d, Database.countQuery('SELECT COUNT() FROM Network'));", count)
					if count == 1 {
						assertions += fmt.Sprintf("Network site = [SELECT Name, UrlPathPrefix FROM Network LIMIT 1]; System.assertEquals('%s', site.Name); System.assertEquals('synthetic', site.UrlPathPrefix);", name)
					}
					class := filepath.Join(root, "force-app/main/default/classes/NetworkGitTest.cls")
					writeTestFile(t, class, "@IsTest private class NetworkGitTest { @IsTest static void selected() {"+assertions+"} }")
					writeTestFile(t, class+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
					runGitCLI(t, root, "init", "--quiet")
					runGitCLI(t, root, "add", ".")
					runGitCLI(t, root, "-c", "user.name=Glade Test", "-c", "user.email=glade@example.test", "-c", "commit.gpgsign=false", "commit", "--quiet", "-m", "synthetic baseline")
					switch operation {
					case "unstaged_edit", "staged_edit":
						writeTestFile(t, path, body+"\n")
						if operation == "staged_edit" {
							runGitCLI(t, root, "add", ".")
						}
					case "deleted":
						if err := os.Remove(path); err != nil {
							t.Fatal(err)
						}
					case "renamed":
						if err := os.Rename(path, filepath.Join(metadataRoot, "networks/Renamed Portal.network")); err != nil {
							t.Fatal(err)
						}
						runGitCLI(t, root, "add", ".")
					case "untracked":
						writeTestFile(t, filepath.Join(metadataRoot, "networks/Another Portal.network"), body)
					}
					for _, route := range []string{"cli_changed_since", "cli_changed_alias", "daemon_changed_since", "daemon_request"} {
						t.Run(route, func(t *testing.T) {
							if strings.HasPrefix(route, "cli_") {
								args := []string{"test", "--changed-since", "HEAD"}
								if route == "cli_changed_alias" {
									args = []string{"test", "changed", "--since", "HEAD"}
								}
								args = append(args, "--project", root, "--json", "--no-progress")
								if route == "cli_changed_since" {
									args = append(args, "--no-serve", "--no-cache", "--read-only")
								}
								code, out, stderr := runPerfCLI(t, context.Background(), args...)
								if code != 0 || !strings.Contains(string(out), `"passed": 1`) {
									t.Fatalf("Git-selected project test exit=%d: %s %s", code, out, stderr)
								}
								return
							}
							daemon, err := testdaemon.New(root)
							if err != nil {
								t.Fatal(err)
							}
							opts := apextest.Options{NoDiskCache: true, Parallelism: 1}
							var run testreport.Run
							if route == "daemon_request" {
								run, _, _, err = daemon.RunRequestV1(context.Background(), testdaemon.NewRunRequestV1(opts, "HEAD", 0, 0, false), nil, nil)
							} else {
								run, _, err = daemon.RunChangedSinceOptions("HEAD", opts)
							}
							if summary := run.Summary(); err != nil || summary.Total != 1 || summary.Passed != 1 {
								t.Fatalf("Git-selected daemon project test = %#v, err=%v, suites=%#v", summary, err, run.Suites)
							}
						})
					}
				})
			}
		}
	}
}
