package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestB3AutoResponseMissingRecipientNativeRoutes(t *testing.T) {
	const fixture = "testdata/conformance/b3_auto_response"
	for _, object := range []string{"Lead", "Case"} {
		for _, api := range []string{"62.0", "67.0"} {
			for _, route := range []string{"anonymous", "istest"} {
				t.Run(object+"/"+api+"/"+route, func(t *testing.T) {
					raw, err := os.ReadFile(filepath.Join(fixture, "extended", "api"+strings.TrimSuffix(api, ".0"), route, "org.tsv"))
					if err != nil {
						t.Fatal(err)
					}
					key := strings.ToLower(object) + "_missing\ttrue,0"
					if !strings.Contains(string(raw), key+"\n") {
						t.Fatalf("missing native row %s", key)
					}
					root := t.TempDir()
					err = filepath.Walk(filepath.Join(fixture, "project"), func(path string, info os.FileInfo, e error) error {
						if e != nil || info.IsDir() {
							return e
						}
						rel, e := filepath.Rel(filepath.Join(fixture, "project"), path)
						if e != nil {
							return e
						}
						body, e := os.ReadFile(path)
						if e != nil {
							return e
						}
						writeFile(t, filepath.Join(root, rel), strings.ReplaceAll(strings.ReplaceAll(string(body), "62.0", api), "67.0", api))
						return nil
					})
					if err != nil {
						t.Fatal(err)
					}
					row := `new Lead(LastName='Synthetic',Company='Synthetic')`
					relationship := "WhoId"
					if object == "Case" {
						row = `new Case(Subject='Synthetic',Status='New',Origin='Web')`
						relationship = "WhatId"
					}
					code := `SObject row=` + row + `;Database.DMLOptions o=new Database.DMLOptions();o.EmailHeader.triggerAutoResponseEmail=true;Database.SaveResult r=Database.insert(row,o);System.assertEquals(true,r.isSuccess());Id rowId=r.Id;System.assertEquals(0,[SELECT COUNT() FROM Task WHERE ` + relationship + `=:rowId]);`
					cls := filepath.Join(root, "force-app/main/default/classes/B3MissingAssertion.cls")
					writeFile(t, cls, `@IsTest private class B3MissingAssertion { @IsTest static void check() { `+code+` } }`)
					writeFile(t, cls+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
					index := loadTestIndex(t, root)
					if route == "istest" {
						report := Run(index, Options{SelectedClasses: []string{"B3MissingAssertion"}, NoDiskCache: true, Parallelism: 1})
						if report.Summary().Passed != 1 {
							t.Fatalf("native %s: %+v", key, report)
						}
						return
					}
					org := orgFromIndex(index)
					org.APIVersion = api
					runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
					program, e := vm.CompileAnonymousWithOptions(code, vm.CompileOptions{APIVersion: api})
					if e != nil {
						t.Fatal(e)
					}
					if _, e = runner.execute(program); e != nil {
						t.Fatalf("native %s: %v", key, e)
					}
				})
			}
		}
	}
}
