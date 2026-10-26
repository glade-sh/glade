package gladecli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestImpactJSONInventoryScratchCumulusOrdinaryChangedAndAliasRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, config := range []string{"config/project-scratch-def.json", "config/hc-project-scratch-def.json", "cumulusci.yml", "cumulusci.template.yml"} {
			for _, route := range []struct {
				name string
				args []string
			}{{"ordinary", []string{"test"}}, {"changed", []string{"test", "--changed-since", "HEAD"}}, {"changed-alias", []string{"test", "changed", "--since", "HEAD"}}} {
				workerCounts := []string{"1", "2"}
				if route.name == "changed-alias" {
					workerCounts = []string{"default"}
				}
				for _, workers := range workerCounts {
					t.Run(api+"/"+config+"/"+route.name+"/workers="+workers, func(t *testing.T) {
						root := writeImpactJSONProject(t, api)
						writeImpactJSONFile(t, filepath.Join(root, "force-app/main/default/classes/ImpactTests.cls"), `@IsTest class ImpactTests { @IsTest static void inventory() { System.assertEquals(7, ImpactWorker.value()); System.assertEquals(true, UserInfo.isMultiCurrencyOrganization()); } }`)
						definition := config
						if config == "cumulusci.yml" || config == "cumulusci.template.yml" {
							definition = "orgs/feature-shape.json"
							writeImpactJSONFile(t, filepath.Join(root, config), "orgs:\n  scratch:\n    dev:\n      config_file: '"+definition+"'\n")
						}
						writeImpactJSONFile(t, filepath.Join(root, definition), `{"features":["MultiCurrency"]}`)
						runGitCLI(t, root, "init", "--quiet")
						runGitCLI(t, root, "add", ".")
						runGitCLI(t, root, "-c", "user.name=Glade Test", "-c", "user.email=glade@example.test", "commit", "--quiet", "-m", "synthetic baseline")
						// Select the test through both changed entry points without
						// relying on their independent config-change policy.
						writeImpactJSONFile(t, filepath.Join(root, "force-app/main/default/classes/ImpactWorker.cls"), `public class ImpactWorker { public static Integer value() { return 3+4; } }`)
						args := append(append([]string{}, route.args...), "--project", root, "--json", "--no-progress")
						if route.name != "changed-alias" {
							args = append(args, "--no-serve", "--no-cache", "--read-only", "--parallelism", workers)
						}
						code, stdout, stderr := runPerfCLI(t, context.Background(), args...)
						identity := perfTestResultIdentity(t, stdout)
						if code != 0 || len(identity) != 1 || identity[0]["status"] != "pass" {
							t.Fatalf("configured runner route did not execute: %d %s %s", code, stdout, stderr)
						}
						path := filepath.Join(t.TempDir(), "impact.json")
						gotCode, out, errOut := runPerfCLI(t, context.Background(), append(args, "--impact-json", path)...)
						if gotCode != code || !reflect.DeepEqual(identity, perfTestResultIdentity(t, out)) || string(stderr) != string(errOut) {
							t.Fatalf("impact changed outcomes: %d %s %s", gotCode, out, errOut)
						}
						data, err := os.ReadFile(path)
						if err != nil {
							t.Fatal(err)
						}
						var artifact impactJSONContract
						if err := json.Unmarshal(data, &artifact); err != nil {
							t.Fatal(err)
						}
						for _, file := range []string{config, definition} {
							if !containsImpactString(artifact.SharedInputs, file) {
								t.Errorf("artifact omits consumed input %s", file)
							}
						}
						assertImpactInventoryIncomplete(t, data)
					})
				}
			}
		}
	}
}

func TestImpactJSONInventoryAbsentMalformedAndSelectorErrorRoutes(t *testing.T) {
	for _, route := range []string{"absent-config", "malformed-config", "selector-error"} {
		t.Run(route, func(t *testing.T) {
			root := writeImpactJSONProject(t, "67.0")
			path := filepath.Join(t.TempDir(), "impact.json")
			args := []string{"test", "--project", root, "--json", "--no-progress", "--no-serve", "--no-cache", "--read-only", "--impact-json", path}
			if route == "malformed-config" {
				writeImpactJSONFile(t, filepath.Join(root, "cumulusci.yml"), "config_file: [invalid\n")
				writeImpactJSONFile(t, filepath.Join(root, "config/project-scratch-def.json"), "{invalid")
			}
			if route == "selector-error" {
				args = append(args, "--class", "MissingInventoryTest")
			}
			_, _, _ = runPerfCLI(t, context.Background(), args...)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			assertImpactInventoryIncomplete(t, data)
		})
	}
}

func assertImpactInventoryIncomplete(t *testing.T, data []byte) {
	t.Helper()
	var artifact struct {
		Complete *bool `json:"inputInventoryComplete"`
		Tests    []struct {
			FallbackReasons []string `json:"fallbackReasons"`
		} `json:"tests"`
	}
	if err := json.Unmarshal(data, &artifact); err != nil {
		t.Fatal(err)
	}
	if artifact.Complete == nil || *artifact.Complete {
		t.Errorf("artifact must explicitly require full-suite selection: %s", data)
	}
	for _, row := range artifact.Tests {
		if !containsImpactString(row.FallbackReasons, "project-input-inventory-incomplete") {
			t.Errorf("row permits unsafe impact selection: %+v", row)
		}
	}
}
