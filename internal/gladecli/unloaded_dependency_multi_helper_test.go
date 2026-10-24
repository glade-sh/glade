package gladecli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnloadedDependencyMultiHelperAnonymousExecNativeRoutes(t *testing.T) {
	dir := filepath.Join("../apextest/testdata/conformance/unloaded_dependency/multi_helper/sources")
	for _, api := range []string{"62.0", "67.0"} {
		for _, count := range []int{2, 3} {
			for _, loaded := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%d_helpers/loaded_%t/anonymous_exec", api, count, loaded), func(t *testing.T) {
					suffix := fmt.Sprintf("%s%d", api[:2], count)
					root := filepath.Join(t.TempDir(), "consumer")
					writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true,"dependencies":[{"package":"cedar-library"}]}]}`)
					files, err := os.ReadDir(filepath.Join(dir, fmt.Sprintf("%d_helpers", count)))
					if err != nil {
						t.Fatal(err)
					}
					for _, file := range files {
						name := strings.ReplaceAll(file.Name(), "SUFFIX", suffix)
						destination := root
						if strings.HasPrefix(name, "CedarLibrary") {
							if !loaded {
								continue
							}
							destination = filepath.Join(filepath.Dir(root), "supplier")
							writeTestFile(t, filepath.Join(destination, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true,"package":"cedar-library"}]}`)
						}
						body, err := os.ReadFile(filepath.Join(dir, fmt.Sprintf("%d_helpers", count), file.Name()))
						if err != nil {
							t.Fatal(err)
						}
						kind, sub := "ApexClass", "classes"
						if strings.HasSuffix(name, ".trigger") {
							kind, sub = "ApexTrigger", "triggers"
						}
						path := filepath.Join(destination, "force-app/main/default", sub, name)
						writeTestFile(t, path, strings.ReplaceAll(string(body), "SUFFIX", suffix))
						writeTestFile(t, path+"-meta.xml", "<"+kind+"><apiVersion>"+api+"</apiVersion><status>Active</status></"+kind+">")
					}
					for _, letter := range "ABC"[:count] + "X" {
						source := fmt.Sprintf("Account row = new Account(Name='%c'); insert row;", letter)
						if loaded && letter != 'X' {
							source += fmt.Sprintf(" System.assertEquals('%c', [SELECT Description FROM Account WHERE Id = :row.Id].Description);", letter)
						}
						var stdout, stderr bytes.Buffer
						code := Run(context.Background(), []string{"exec", "--project", root, "--json", source}, &stdout, &stderr)
						output := stdout.String() + stderr.String()
						if loaded || letter == 'X' {
							if code != 0 {
								t.Errorf("loaded/boundary %c: code=%d %s", letter, code, output)
							}
						} else if code == 0 || !strings.Contains(output, "dependency_missing") || !strings.Contains(output, "CedarLibrary"+string(letter)) || !strings.Contains(output, "cedar-library") {
							t.Errorf("reached helper %c: want dependency_missing at library: code=%d %s", letter, code, output)
						}
					}
				})
			}
		}
	}
}
