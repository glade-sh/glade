package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// organizationId identifies the organization in which installation occurs.
// https://developer.salesforce.com/docs/platform/pkg1-dev/guide/apex-post-install-script-intro.html
func TestInstallContextOrganizationIDMatchesActiveOrg(t *testing.T) {
	for _, api := range []string{"48.0", "62.0", "67.0"} {
		for _, install := range []struct{ name, arguments string }{
			{"fresh", "null"},
			{"upgrade", "new Version(1, 2, 3), false"},
		} {
			t.Run(api+"/"+install.name, func(t *testing.T) {
				executeInstallOrganizationProbe(t, api, install.arguments, `
Id installedOrg=context.organizationId();
System.assertNotEquals(null,installedOrg);
System.assertEquals(UserInfo.getOrganizationId(),installedOrg);
System.assertEquals(installedOrg,context.OrganizationId());
`)
			})
		}
	}
}

func TestInstallContextOrganizationIDRetainsVersionControls(t *testing.T) {
	for _, api := range []string{"48.0", "62.0", "67.0"} {
		for _, install := range []struct{ name, arguments, assertions string }{
			{"fresh", "null", `System.assertEquals(null,context.previousVersion()); System.assertEquals(false,context.isUpgrade());`},
			{"upgrade", "new Version(1, 2, 3), false", `System.assertEquals('1.2.3',context.previousVersion().toString()); System.assertEquals(true,context.isUpgrade());`},
		} {
			t.Run(api+"/"+install.name, func(t *testing.T) {
				executeInstallOrganizationProbe(t, api, install.arguments, install.assertions+`
System.assertEquals(false,context.isPush());
System.assertEquals(UserInfo.getUserId(),context.installerId());
`)
			})
		}
	}
}

func executeInstallOrganizationProbe(t *testing.T, api, arguments, body string) {
	t.Helper()
	program, err := CompileAnonymousWithOptions(fmt.Sprintf("Test.testInstall(new InstallOrganizationProbe(), %s);", arguments), CompileOptions{APIVersion: api})
	if err != nil {
		t.Fatal(err)
	}
	onInstall, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: api})
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.OrgID = "00D000000000001"
	machine.SetOrg(&org)
	machine.SetCurrentUser(storage.Record{ID: "005000000000999", Object: "User"})
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name: "InstallOrganizationProbe", APIVersion: api, Interfaces: []string{"InstallHandler"},
		Methods: map[string]Method{"onInstall": {
			Name: "InstallOrganizationProbe.onInstall", ClassName: "InstallOrganizationProbe", APIVersion: api, SourceContextBound: true, ReturnType: "void",
			Params: []Param{{Name: "context", Type: "InstallContext"}}, Program: onInstall,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
