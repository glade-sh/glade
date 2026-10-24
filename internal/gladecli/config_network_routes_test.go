package gladecli

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigNetworkAnonymousAndTestRunnerRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, shape := range []string{"disabled", "enabled", "extra_metadata"} {
			t.Run(api+"/"+shape, func(t *testing.T) {
				root := t.TempDir()
				writeTestFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
				cfg := ""
				available := "false"
				count := 0
				if shape != "disabled" {
					cfg = "org:\n  features: [Communities]\n"
					available = "true"
				}
				if shape == "extra_metadata" {
					cfg += "project:\n  extraMetadataDirs: [supplemental]\n"
					count = 1
					writeTestFile(t, filepath.Join(root, "supplemental/networks/Synthetic Portal.network-meta.xml"), `<Network><status>UnderConstruction</status><urlPathPrefix>synthetic</urlPathPrefix></Network>`)
				}
				writeTestFile(t, filepath.Join(root, "glade.yml"), cfg)
				body := "System.assertEquals(" + available + ", Schema.getGlobalDescribe().containsKey('Network'));"
				body += "System.assertEquals(" + available + ", Type.forName('Schema.Network') != null); System.assertNotEquals(null, Type.forName('Network')); System.assertEquals(null, Type.forName('Schema','Network'));"
				if shape != "disabled" {
					body += fmt.Sprintf("System.assertEquals(%d, Database.countQuery('SELECT COUNT() FROM Network'));", count)
					body += `Boolean caught=false; try { SObject record=Database.query('SELECT Id FROM Network WHERE Name = \'Missing Synthetic Portal\''); } catch(QueryException e) { caught=true; } System.assert(caught);`
				}
				writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/NetworkConfigProbe.cls"), "public class NetworkConfigProbe { public static void checkShape() {"+body+"} }")
				writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/NetworkConfigProbeTest.cls"), "@IsTest private class NetworkConfigProbeTest { @IsTest static void named() { NetworkConfigProbe.checkShape(); } }")
				for _, name := range []string{"NetworkConfigProbe", "NetworkConfigProbeTest"} {
					writeTestFile(t, filepath.Join(root, "force-app/main/default/classes", name+".cls-meta.xml"), "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
				}
				run := func(t *testing.T, args ...string) string {
					t.Helper()
					var out, stderr bytes.Buffer
					code := Run(context.Background(), args, &out, &stderr)
					if code != 0 {
						t.Fatalf("%v exit=%d: %s%s", args[0], code, out.String(), stderr.String())
					}
					return out.String()
				}
				t.Run("config_validate", func(t *testing.T) { run(t, "config", "validate", "--project", root) })
				t.Run("config_show", func(t *testing.T) {
					out := run(t, "config", "show", "--project", root, "--json")
					if shape == "extra_metadata" && !strings.Contains(out, `"extraMetadataDirs"`) {
						t.Fatalf("extra directories omitted: %s", out)
					}
				})
				t.Run("anonymous", func(t *testing.T) { run(t, "exec", "--project", root, "--json", "NetworkConfigProbe.checkShape();") })
				t.Run("named_isTest_runner", func(t *testing.T) {
					out := run(t, "test", "--project", root, "--class", "NetworkConfigProbeTest", "--json", "--no-progress", "--no-cache", "--no-serve")
					if !strings.Contains(out, `"passed": 1`) {
						t.Fatalf("missing passing test: %s", out)
					}
				})
				t.Run("named_isTest_cached_runner", func(t *testing.T) {
					for i := 0; i < 2; i++ {
						out := run(t, "test", "--project", root, "--class", "NetworkConfigProbeTest", "--json", "--no-progress", "--no-serve")
						if !strings.Contains(out, `"passed": 1`) {
							t.Fatalf("missing passing cached-route test: %s", out)
						}
					}
				})
			})
		}
	}
}
