package lsp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestPersonAccountEvidenceLSPRoutes(t *testing.T) {
	const dir = "../apextest/testdata/conformance/person_account_evidence"
	raw, err := os.ReadFile(filepath.Join(dir, "evidence.json"))
	if err != nil {
		t.Fatal(err)
	}
	var shapes []struct {
		Name    string
		Enabled bool
		Files   map[string]string
	}
	if err := json.Unmarshal(raw, &shapes); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "observe.apex"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, shape := range shapes {
			for _, route := range []string{"named", "isTest"} {
				t.Run(api+"/"+shape.Name+"/"+route, func(t *testing.T) {
					root := t.TempDir()
					write := func(path, source string) {
						t.Helper()
						path = filepath.Join(root, path)
						if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(path, []byte(source), 0600); err != nil {
							t.Fatal(err)
						}
					}
					write("sfdx-project.json", `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
					write("force-app/main/default/objects/Account/Account.object-meta.xml", `<CustomObject><fields><fullName>Name</fullName><type>Text</type></fields></CustomObject>`)
					for path, source := range shape.Files {
						write(path, source)
					}
					path := "force-app/main/default/classes/PersonEvidence.cls"
					source := "public class PersonEvidence { public static void sample(){" + string(body) + "} }"
					if route == "isTest" {
						source = "@IsTest private class PersonEvidence { @IsTest static void sample(){" + string(body) + "} }"
					}
					write(path, source)
					write(path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
					p, err := project.Load(root)
					if err != nil {
						t.Fatal(err)
					}
					s, err := schema.LoadProject(p)
					if err != nil {
						t.Fatal(err)
					}
					index := typesys.Build(p, s)
					if index.HasErrors() {
						t.Fatal(index.Diagnostics)
					}
					handler := NewHandler(index)
					defer handler.parser.Close()
					var messages []string
					for _, d := range handler.diagnosticsForDocument(uriFromPath(filepath.Join(root, path))) {
						if d.Severity == diagnostic.Error {
							messages = append(messages, d.Message)
						}
					}
					if (len(messages) == 0) != shape.Enabled {
						t.Fatalf("enabled=%t; LSP errors=%v", shape.Enabled, messages)
					}
					if !shape.Enabled && !strings.Contains(strings.Join(messages, " "), "FirstName") {
						t.Fatalf("missing native field rejection: %v", messages)
					}
				})
			}
		}
	}
}
