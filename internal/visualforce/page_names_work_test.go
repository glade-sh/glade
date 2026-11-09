package visualforce

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// writePageNamesWorkProject writes classes Apex classes and pages Visualforce
// pages with controllers; the last page binds an unknown property so that
// PageNames takes the per-page fallback.
func writePageNamesWorkProject(t *testing.T, root string, classes, pages int) project.Project {
	t.Helper()
	classDir := filepath.Join(root, "force-app", "main", "default", "classes")
	pageDir := filepath.Join(root, "force-app", "main", "default", "pages")
	for _, dir := range []string{classDir, pageDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(path, text string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion": "62.0", "packageDirectories": [{"path": "force-app", "default": true}]}`)
	classMeta := `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>`
	for i := 0; i < classes; i++ {
		name := fmt.Sprintf("WorkClass%d", i)
		write(filepath.Join(classDir, name+".cls"), fmt.Sprintf(`public with sharing class %s {
    public String name { get; set; }
    public %s() { name = 'n%d'; }
    public PageReference save() { return null; }
}
`, name, name, i))
		write(filepath.Join(classDir, name+".cls-meta.xml"), classMeta)
	}
	for j := 0; j < pages; j++ {
		name := fmt.Sprintf("WorkPage%d", j)
		expression := "{!name}"
		if j == pages-1 {
			expression = "{!noSuchProperty}"
		}
		write(filepath.Join(pageDir, name+".page"), fmt.Sprintf(`<apex:page controller="WorkClass%d">
  <apex:form>
    <apex:outputText value="%s"/>
    <apex:commandButton action="{!save}" value="Save"/>
  </apex:form>
</apex:page>
`, j%classes, expression))
		write(filepath.Join(pageDir, name+".page-meta.xml"), fmt.Sprintf(`<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><label>%s</label></ApexPage>`, name))
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// TestPageNamesFallbackWorkIsLinearInClasses pins the cost of registering the
// pages of a project with one invalid page: the per-page fallback must parse
// each Apex file once and compile the project once, not once per page.
// Doubling the page count must not change the Apex work.
func TestPageNamesFallbackWorkIsLinearInClasses(t *testing.T) {
	const classes = 12
	measure := func(pages int) (names int, rejected int, parses, compiles uint64) {
		p := writePageNamesWorkProject(t, t.TempDir(), classes, pages)
		parsesBefore, compilesBefore := apexSourceParses.Load(), controllerCompiles.Load()
		got, left := PageNamesWithDiagnostics(p, nil)
		return len(got), len(left), apexSourceParses.Load() - parsesBefore, controllerCompiles.Load() - compilesBefore
	}
	for _, pages := range []int{4, 8} {
		names, rejected, parses, compiles := measure(pages)
		if names != pages-1 || rejected != 1 {
			t.Fatalf("pages=%d: got %d names and %d rejections, want %d and 1", pages, names, rejected, pages-1)
		}
		// One parse per class for the expression scan; the compile gate's
		// syntax-recovery parse of a rejected file is not triggered here.
		if parses > classes {
			t.Fatalf("pages=%d: parsed Apex %d times for %d classes; the per-page fallback re-parses the project", pages, parses, classes)
		}
		if compiles > 1 {
			t.Fatalf("pages=%d: compiled the project %d times; the per-page fallback recompiles", pages, compiles)
		}
	}
}

// TestLoadProjectWithoutCacheRecomputes keeps the nil-cache path honest: a
// direct LoadProject still parses the Apex sources once.
func TestLoadProjectWithoutCacheRecomputes(t *testing.T) {
	p := writePageNamesWorkProject(t, t.TempDir(), 3, 1)
	before := apexSourceParses.Load()
	if _, err := LoadProject(p); err == nil {
		t.Fatal("expected the unknown-property page to fail a strict load")
	}
	if apexSourceParses.Load()-before != 3 {
		t.Fatalf("LoadProject parsed Apex %d times, want 3", apexSourceParses.Load()-before)
	}
}
