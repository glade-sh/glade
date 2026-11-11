package visualforce

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// A prepared debugger session must not read project metadata until a page is
// actually requested, and must use the loaded project for the first render.
func TestLazyRenderEnvironmentLoadsOnFirstPage(t *testing.T) {
	root := filepath.Join(t.TempDir(), "project")
	machine := vm.New(nil)
	SetVMLazyRenderEnvironment(machine, root)
	defer ClearVMRenderEnvironment(machine)
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/LazyPage.page"), `<apex:page showHeader="false" sidebar="false">Lazy page</apex:page>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/LazyPage.page-meta.xml"), `<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><label>Lazy Page</label></ApexPage>`)
	body, err := RenderPageURL(machine, "/apex/LazyPage", false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body.Fields["value"].Text, "Lazy page") {
		t.Fatalf("unexpected page: %#v", body)
	}
}
