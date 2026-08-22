package enterprisegraph

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/enterprise"
)

func TestDetectFFLibInventoryDetectsDomainSelectorServiceAndFactory(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "sfdx-project.json", `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"65.0"}`)
	classes := filepath.Join(root, "force-app", "main", "default", "classes")
	writeTestFile(t, classes, "AccountDomain.cls", `public class AccountDomain extends fflib_SObjectDomain {}`)
	writeTestFile(t, classes, "AccountSelector.cls", `public class AccountSelector implements fflib_ISObjectSelector {}`)
	writeTestFile(t, classes, "AccountService.cls", `public class AccountService { static void go(){ fflib_Application.Service.newInstance(AccountService.class); } }`)
	writeTestFile(t, classes, "ApplicationFactory.cls", `public class ApplicationFactory { fflib_Application.SelectorFactory selectors; fflib_ISObjectUnitOfWork uow; }`)

	ctx, err := enterprise.LoadContext(root)
	if err != nil {
		t.Fatalf("LoadContext() error = %v", err)
	}

	inv := DetectFFLib(ctx)

	if !contains(inv.Domains, "AccountDomain") {
		t.Fatalf("domains = %v, want AccountDomain", inv.Domains)
	}
	if !contains(inv.Selectors, "AccountSelector") {
		t.Fatalf("selectors = %v, want AccountSelector", inv.Selectors)
	}
	if !contains(inv.Services, "AccountService") {
		t.Fatalf("services = %v, want AccountService", inv.Services)
	}
	if !contains(inv.UnitOfWorkUsers, "ApplicationFactory") {
		t.Fatalf("unit of work users = %v, want ApplicationFactory", inv.UnitOfWorkUsers)
	}
	if !contains(inv.Factories, "ApplicationFactory") {
		t.Fatalf("factories = %v, want ApplicationFactory", inv.Factories)
	}
}
