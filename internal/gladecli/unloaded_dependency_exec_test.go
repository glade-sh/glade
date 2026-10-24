package gladecli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnloadedDependencyAnonymousExecNativeRoutes(t *testing.T) {
	dir := filepath.Join("..", "apextest", "testdata", "conformance", "unloaded_dependency")
	var cases []struct {
		ID, State, Source, Outcome string
		Trigger                    bool
	}
	raw, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	// The same native Apex slice also covers a local package identity matching
	// the unresolved qualifier, which prepends a resolver problem in the gate.
	for _, cell := range append(cases[:0:0], cases...) {
		if cell.State == "absent" && cell.Outcome == "compile-rejected" {
			twin := cell
			twin.ID += "_resolver_first"
			cases = append(cases, twin)
		}
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, cell := range cases {
			t.Run(api+"/"+cell.ID+"/anonymous_exec", func(t *testing.T) {
				dependencyName := "cedar-library"
				if strings.HasSuffix(cell.ID, "_resolver_first") {
					dependencyName = "CedarLibrary" + api[:2]
				}
				dependencies := `[{"package":"` + dependencyName + `"}]`
				if cell.ID == "member_rejection" {
					dependencies = `[{"package":"cedar-library"},{"package":"cedar-optional"}]`
				}
				workspace := t.TempDir()
				root := filepath.Join(workspace, "consumer")
				writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true,"dependencies":`+dependencies+`}]}`)
				subst := func(s string) string {
					return strings.ReplaceAll(strings.ReplaceAll(s, "CedarLibrary", "CedarLibrary"+api[:2]), "CedarGate", "CedarGate"+api[:2])
				}
				if cell.State == "loaded" {
					supplier := filepath.Join(workspace, "supplier")
					writeTestFile(t, filepath.Join(supplier, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true,"package":"cedar-library"}]}`)
					body, err := os.ReadFile(filepath.Join(dir, "sources/CedarLibrary.cls"))
					if err != nil {
						t.Fatal(err)
					}
					path := filepath.Join(supplier, "force-app/main/default/classes", "CedarLibrary"+api[:2]+".cls")
					writeTestFile(t, path, subst(string(body)))
					writeTestFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
				}
				if cell.Trigger {
					body, err := os.ReadFile(filepath.Join(dir, "sources/CedarGate.trigger"))
					if err != nil {
						t.Fatal(err)
					}
					path := filepath.Join(root, "force-app/main/default/triggers", "CedarGate"+api[:2]+".trigger")
					writeTestFile(t, path, subst(string(body)))
					writeTestFile(t, path+"-meta.xml", "<ApexTrigger><apiVersion>"+api+"</apiVersion><status>Active</status></ApexTrigger>")
				}
				var stdout, stderr bytes.Buffer
				code := Run(context.Background(), []string{"exec", "--project", root, "--json", subst(cell.Source)}, &stdout, &stderr)
				output := stdout.String() + stderr.String()
				if cell.Outcome == "accepted" {
					if code != 0 {
						t.Fatalf("native accepted: code=%d %s", code, output)
					}
					return
				}
				if code == 0 {
					t.Fatal("native rejected; exec succeeded")
				}
				if cell.State == "loaded" {
					if strings.Contains(output, "dependency_missing") {
						t.Fatalf("known-class rejection relabeled: %s", output)
					}
					return
				}
				if strings.Contains(output, "dependency_missing: dependency_missing:") {
					t.Fatalf("duplicate dependency diagnostic prefix: %s", output)
				}
				if !strings.Contains(output, "dependency_missing") || !strings.Contains(output, dependencyName) {
					t.Fatalf("want dependency_missing with unloaded package: %s", output)
				}
			})
		}
	}
}

func TestUnloadedDependencyKnownPlatformAnonymousPolicy(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api+"/anonymous_exec", func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true,"dependencies":[{"package":"cedar-optional"}]}]}`)
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), []string{"exec", "--project", root, "--json", `System.String.absentMember();`}, &stdout, &stderr)
			output := stdout.String() + stderr.String()
			if code == 0 || strings.Contains(output, "dependency_missing") {
				t.Fatalf("known platform type relabeled: code=%d %s", code, output)
			}
		})
	}
}
