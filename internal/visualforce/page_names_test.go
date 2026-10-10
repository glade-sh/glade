package visualforce_test

import (
	"fmt"
	"os"
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

// These controls preserve the existing strict loader's registration behavior;
// they do not establish Salesforce acceptance for the fixture sources.
func TestPageNamesPreserveStrictLoaderRegistration(t *testing.T) {
	const controller = `public class PageController { public String getMessage() { return 'ready'; } }`
	const metadata = `<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><label>Example</label></ApexPage>`
	cases := []struct {
		name      string
		files     map[string]string
		wantNames []string
		wantError bool
	}{
		{
			name:      "valid_page",
			files:     map[string]string{"pages/Example.page": `<apex:page>Ready</apex:page>`},
			wantNames: []string{"Example"},
		},
		{
			name: "valid_controller_expression",
			files: map[string]string{
				"classes/PageController.cls": controller,
				"pages/Example.page":         `<apex:page controller="PageController"><apex:outputText value="{!message}"/></apex:page>`,
			},
			wantNames: []string{"Example"},
		},
		{
			name: "discovered_names_and_sidecars",
			files: map[string]string{
				"pages/Example.page":               `<apex:page/>`,
				"pages/Example.page-meta.xml":      metadata,
				"pages/nested/Alpha.PAGE":          `<apex:page/>`,
				"pages/MetadataOnly.page-meta.xml": metadata,
				"pages/zpkg__Order.page":           `<apex:page/>`,
				"pages/Ignored.txt":                "not a page",
			},
			wantNames: []string{"Alpha", "Example", "MetadataOnly", "zpkg__Order"},
		},
		{
			name: "invalid_controller_expression",
			files: map[string]string{
				"classes/PageController.cls": controller,
				"pages/Example.page":         `<apex:page controller="PageController"><apex:outputText value="{!missing}"/></apex:page>`,
			},
			wantError: true,
		},
		{
			name: "invalid_metadata",
			files: map[string]string{
				"pages/Example.page":          `<apex:page/>`,
				"pages/Example.page-meta.xml": `<ApexPage><apiVersion>invalid</apiVersion></ApexPage>`,
			},
			wantError: true,
		},
		{
			name: "invalid_sibling_omits_all_names",
			files: map[string]string{
				"classes/PageController.cls": controller,
				"components/Panel.component": `<apex:component/>`,
				"pages/Example.page":         `<apex:page/>`,
				"pages/Invalid.page":         `<apex:page><apex:form></apex:page>`,
			},
			wantError: true,
		},
		{
			name: "invalid_component_structure",
			files: map[string]string{
				"pages/Example.page":         `<apex:page/>`,
				"components/Panel.component": `<apex:component><apex:outputText></apex:component>`,
			},
			wantError: true,
		},
		{
			name: "invalid_component_type",
			files: map[string]string{
				"pages/Example.page":         `<apex:page/>`,
				"components/Panel.component": `<apex:component><apex:attribute name="value" type="MissingType" description="Value"/></apex:component>`,
			},
			wantError: true,
		},
	}
	for _, api := range []string{"59.0", "67.0"} {
		for _, tc := range cases {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				root := t.TempDir()
				writePageNameFixture(t, root, "sfdx-project.json", fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"pkg","sourceApiVersion":%q}`, api))
				for name, source := range tc.files {
					if strings.Contains(source, "%s") {
						source = fmt.Sprintf(source, api)
					}
					writePageNameFixture(t, root, "force-app/main/default/"+name, source)
				}
				p, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				strict, err := visualforce.LoadProject(p)
				if (err != nil) != tc.wantError {
					t.Fatalf("strict LoadProject error = %v, want rejection = %t", err, tc.wantError)
				}
				var strictNames []string
				for _, page := range strict.Pages {
					strictNames = append(strictNames, page.Name)
				}
				if !slices.Equal(strictNames, tc.wantNames) {
					t.Fatalf("strict page names = %v, want %v", strictNames, tc.wantNames)
				}

				// Feed an unsorted discovery slice to check sorting without mutation.
				slices.Reverse(p.VisualforcePageFiles)
				paths := slices.Clone(p.VisualforcePageFiles)
				if got := visualforce.PageNames(p); !slices.Equal(got, tc.wantNames) {
					t.Errorf("page names = %v, want %v", got, tc.wantNames)
				}
				if !slices.Equal(p.VisualforcePageFiles, paths) {
					t.Error("PageNames changed the discovered paths")
				}

				var index typesys.Index
				index.Project.Root = root
				runtime, err := apextest.CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
				if err != nil {
					t.Fatal(err)
				}
				var wantRuntimeNames []string
				for _, name := range tc.wantNames {
					wantRuntimeNames = append(wantRuntimeNames, name)
					if !strings.Contains(name, "__") {
						wantRuntimeNames = append(wantRuntimeNames, "pkg__"+name)
					}
				}
				if !slices.Equal(runtime.PageNames, wantRuntimeNames) {
					t.Errorf("runtime page names = %v, want %v", runtime.PageNames, wantRuntimeNames)
				}
				machine := vm.New(nil)
				if err := apextest.RegisterCompiledProjectRuntimeForRequest(machine, runtime); err != nil {
					t.Fatal(err)
				}
				wantURL := "Page.Example"
				if !tc.wantError {
					wantURL = "/apex/Example"
				}
				program, err := vm.CompileAnonymousWithOptions(fmt.Sprintf("System.assertEquals('%s', new PageReference('Page.Example').getUrl());", wantURL), vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.Execute(program); err != nil {
					t.Errorf("PageReference getUrl: %v", err)
				}
				program, err = vm.CompileAnonymousWithOptions("System.assertEquals('/apex/Missing', Page.Missing.getUrl());", vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				_, err = machine.Execute(program)
				if tc.wantError {
					if err != nil {
						t.Errorf("unknown page with empty registry: %v", err)
					}
				} else if err == nil {
					t.Error("unknown page with populated registry unexpectedly succeeded")
				}
			})
		}
	}
}

func writePageNameFixture(t *testing.T, root, name, source string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
}
