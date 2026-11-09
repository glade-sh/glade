package visualforce_test

import (
	"fmt"
	"path/filepath"
	"slices"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/visualforce"
)

// A static resource from a loaded source dependency (glade.yml
// dependencyPaths) is installed in the org with that dependency, so a root page
// whose apex:stylesheet names it registers. A page whose stylesheet names a
// resource no project declares is rejected alone: it does not take the
// project's other pages, in nested folders with -meta.xml files, with it.
func TestPageNamesDependencyStylesheetResource(t *testing.T) {
	resourceMeta := `<?xml version="1.0" encoding="UTF-8"?><StaticResource xmlns="http://soap.sforce.com/2006/04/metadata"><cacheControl>Public</cacheControl><contentType>text/css</contentType></StaticResource>`
	pageMeta := func(label string) string {
		return `<?xml version="1.0" encoding="UTF-8"?><ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>59.0</apiVersion><label>` + label + `</label></ApexPage>`
	}
	styled := func(resource string) string {
		return `<apex:page renderAs="pdf" showHeader="false"><apex:stylesheet value="{!$Resource.` + resource + `}"/>Styled</apex:page>`
	}
	for _, api := range []string{"59.0", "67.0"} {
		for _, tc := range []struct {
			name     string
			resource string
			want     []string
		}{
			{"dependency_resource", "Cedar_Shared_Style", []string{"CedarConfirm", "CedarStyled"}},
			{"missing_resource", "Cedar_Absent_Style", []string{"CedarConfirm"}},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				root := t.TempDir()
				consumer := filepath.Join(root, "consumer")
				base := filepath.Join(root, "base")
				writePageNameFixture(t, consumer, "sfdx-project.json", fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true,"dependencies":[{"package":"CedarBase"}]}],"sourceApiVersion":%q}`, api))
				writePageNameFixture(t, consumer, "glade.yml", "project:\n  dependencyPaths: [\"../base\"]\n")
				files := map[string]string{
					"force-app/main/default/pages/CedarStyled.page":             styled(tc.resource),
					"force-app/main/default/pages/CedarStyled.page-meta.xml":    pageMeta("CedarStyled"),
					"force-app/community/site/pages/CedarConfirm.page":          `<apex:page showHeader="false">Confirmed</apex:page>`,
					"force-app/community/site/pages/CedarConfirm.page-meta.xml": pageMeta("CedarConfirm"),
				}
				for name, source := range files {
					writePageNameFixture(t, consumer, filepath.FromSlash(name), source)
				}
				writePageNameFixture(t, base, "sfdx-project.json", fmt.Sprintf(`{"packageDirectories":[{"path":"base-app","default":true,"package":"CedarBase"}],"sourceApiVersion":%q}`, api))
				writePageNameFixture(t, base, "base-app/main/default/staticresources/Cedar_Shared_Style.css", "body { color: black; }")
				writePageNameFixture(t, base, "base-app/main/default/staticresources/Cedar_Shared_Style.resource-meta.xml", resourceMeta)
				p, err := project.Load(consumer)
				if err != nil {
					t.Fatal(err)
				}
				names, rejected := visualforce.PageNamesWithDiagnostics(p, nil)
				if !slices.Equal(names, tc.want) {
					t.Fatalf("page names = %v, want %v; rejected %+v", names, tc.want, rejected)
				}
			})
		}
	}
}
