package visualforce_test

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/visualforce"
	"github.com/glade-sh/glade/internal/vm"
)

// One page that cannot load, or metadata that names a page the project lacks,
// must not make Page.Good unknown. The strict loader still rejects the project
// and PageNamesWithDiagnostics names what it left out.
func TestPageNamesKeepLoadablePagesWhenAnotherFails(t *testing.T) {
	const controller = `public class GoodController { public String getMessage() { return 'ready'; } }`
	cases := []struct {
		name         string
		files        map[string]string
		rejectedFile string // suffix of the file the diagnostics must name
	}{
		{
			name: "unparseable_sibling",
			files: map[string]string{
				"pages/Good.page": `<apex:page>Ready</apex:page>`,
				"pages/Bad.page":  `<apex:page><apex:form></apex:page>`,
			},
			rejectedFile: "Bad.page",
		},
		{
			name: "sibling_with_unknown_expression",
			files: map[string]string{
				"classes/GoodController.cls": controller,
				"pages/Good.page":            `<apex:page controller="GoodController">{!message}</apex:page>`,
				"pages/Bad.page":             `<apex:page controller="GoodController">{!missing}</apex:page>`,
			},
			rejectedFile: "Bad.page",
		},
		{
			name: "sibling_using_a_broken_component",
			files: map[string]string{
				"pages/Good.page":             `<apex:page>Ready</apex:page>`,
				"pages/Bad.page":              `<apex:page><c:Broken/></apex:page>`,
				"components/Broken.component": `<apex:component><apex:attribute name="value" type="MissingType" description="Value"/></apex:component>`,
			},
			rejectedFile: "Bad.page",
		},
		{
			name: "permission_set_names_a_missing_page",
			files: map[string]string{
				"pages/Good.page": `<apex:page>Ready</apex:page>`,
				"permissionsets/Access.permissionset-meta.xml": `<PermissionSet xmlns="http://soap.sforce.com/2006/04/metadata"><label>Access</label>` +
					`<pageAccesses><apexPage>Elsewhere</apexPage><enabled>true</enabled></pageAccesses></PermissionSet>`,
			},
			rejectedFile: "Access.permissionset-meta.xml",
		},
	}
	for _, api := range []string{"59.0", "67.0"} {
		for _, tc := range cases {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				root := t.TempDir()
				writePageNameFixture(t, root, "sfdx-project.json", fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
				for name, source := range tc.files {
					writePageNameFixture(t, root, "force-app/main/default/"+name, source)
				}
				p, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := visualforce.LoadProject(p); err == nil {
					t.Fatal("strict LoadProject accepted the project; the fixture must stay rejected")
				}
				names, rejected := visualforce.PageNamesWithDiagnostics(p, nil)
				if !slices.Equal(names, []string{"Good"}) {
					t.Fatalf("page names = %v, want [Good]", names)
				}
				found := false
				for _, r := range rejected {
					if strings.HasSuffix(filepath.ToSlash(r.File), tc.rejectedFile) && r.Message != "" {
						found = true
					}
				}
				if !found {
					t.Fatalf("no diagnostic for %s in %+v", tc.rejectedFile, rejected)
				}

				var index typesys.Index
				index.Project.Root = root
				runtime, err := apextest.CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
				if err != nil {
					t.Fatal(err)
				}
				machine := vm.New(nil)
				if err := apextest.RegisterCompiledProjectRuntimeForRequest(machine, runtime); err != nil {
					t.Fatal(err)
				}
				run := func(source string) error {
					program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					_, err = machine.Execute(program)
					return err
				}
				if err := run("System.assert('/apex/Good'.equalsIgnoreCase(Page.Good.getUrl()), Page.Good.getUrl());"); err != nil {
					t.Errorf("Page.Good.getUrl(): %v", err)
				}
				if err := run("System.assert('/apex/Bad'.equalsIgnoreCase(Page.Bad.getUrl()), Page.Bad.getUrl());"); err == nil && tc.rejectedFile == "Bad.page" {
					t.Error("the rejected page resolved")
				}
			})
		}
	}
}

// A page that links to another page with $Page keeps loading when a third
// page fails: the page-by-page fallback validates one page at a time but still
// resolves $Page references against every page in the project.
func TestPageNamesKeepCrossPageReferencesWhenAnotherFails(t *testing.T) {
	for _, api := range []string{"59.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writePageNameFixture(t, root, "sfdx-project.json", fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			files := map[string]string{
				"classes/GoodController.cls": `public class GoodController { public String getMessage() { return 'ready'; } }`,
				"pages/Linked.page":          `<apex:page controller="GoodController">{!message} <apex:outputLink value="{!$Page.Target}">go</apex:outputLink></apex:page>`,
				"pages/Target.page":          `<apex:page controller="GoodController">{!message} <apex:outputLink value="{!$Page.Linked}">back</apex:outputLink></apex:page>`,
				"pages/Bad.page":             `<apex:page controller="GoodController">{!missing}</apex:page>`,
			}
			for name, source := range files {
				writePageNameFixture(t, root, "force-app/main/default/"+name, source)
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := visualforce.LoadProject(p); err == nil {
				t.Fatal("strict LoadProject accepted the project; the fixture must stay rejected")
			}
			names, rejected := visualforce.PageNamesWithDiagnostics(p, nil)
			if !slices.Equal(names, []string{"Linked", "Target"}) {
				t.Fatalf("page names = %v, want [Linked Target]; rejected=%+v", names, rejected)
			}
			if len(rejected) != 1 || !strings.HasSuffix(filepath.ToSlash(rejected[0].File), "Bad.page") {
				t.Fatalf("rejected = %+v, want only Bad.page", rejected)
			}
		})
	}
}
