package visualforce_test

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/visualforce"
)

func TestControllerlessActionNativeRows(t *testing.T) {
	read := func(name string) [][]string {
		t.Helper()
		f, err := os.Open(filepath.Join("testdata", "vf_controllerless_action", name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		r := csv.NewReader(f)
		r.Comma = '\t'
		r.LazyQuotes = true
		rows, err := r.ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		return rows[1:]
	}
	cases := map[string]string{}
	for _, row := range read("cases.tsv") {
		cases[row[0]] = row[1]
	}
	for _, row := range read("org.tsv") {
		api, name, outcome := row[0], row[1], row[2]
		t.Run(api+"/"+name, func(t *testing.T) {
			root := t.TempDir()
			pages := filepath.Join(root, "force-app", "main", "default", "pages")
			if err := os.MkdirAll(pages, 0755); err != nil {
				t.Fatal(err)
			}
			write := func(path, text string) {
				t.Helper()
				if err := os.WriteFile(path, []byte(text), 0644); err != nil {
					t.Fatal(err)
				}
			}
			write(filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			write(filepath.Join(pages, "Probe.page"), cases[name])
			write(filepath.Join(pages, "Probe.page-meta.xml"), fmt.Sprintf(`<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><label>Probe</label></ApexPage>`, api))
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			_, err = visualforce.LoadProject(p)
			if outcome == "accept" && err != nil {
				t.Fatalf("native accepted: %v", err)
			}
			if outcome == "reject" && err == nil {
				t.Fatalf("native rejected %s; Glade accepted", strings.TrimSpace(cases[name]))
			}
		})
	}
}

// ActionCustomNewBarePg in vf_component_ids/org.tsv accepts this shape at
// source 59.0 and 67.0, including when the project contains no Apex classes.
func TestControllerlessActionWithDeclaredCustomObject(t *testing.T) {
	for _, api := range []string{"59.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			pageDir := filepath.Join(root, "force-app", "main", "default", "pages")
			objectDir := filepath.Join(root, "force-app", "main", "default", "objects", "CedarObj__c")
			for _, dir := range []string{pageDir, objectDir} {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
			}
			write := func(path, text string) {
				t.Helper()
				if err := os.WriteFile(path, []byte(text), 0644); err != nil {
					t.Fatal(err)
				}
			}
			write(filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			write(filepath.Join(pageDir, "Probe.page"), `<apex:page><apex:outputLink value="{!URLFOR($Action.CedarObj__c.New)}">x</apex:outputLink></apex:page>`)
			write(filepath.Join(pageDir, "Probe.page-meta.xml"), fmt.Sprintf(`<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><label>Probe</label></ApexPage>`, api))
			write(filepath.Join(objectDir, "CedarObj__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Cedar</label><pluralLabel>Cedars</pluralLabel><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel><nameField><label>Name</label><type>Text</type></nameField></CustomObject>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := visualforce.LoadProject(p); err != nil {
				t.Fatalf("native accepted: %v", err)
			}
		})
	}
}
