package vm

import (
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func TestPlainDeveloperPicklistPreservesConfiguredValues(t *testing.T) {
	field := storage.Field{APIName: "Type", PicklistValuesConfigured: true, PicklistValues: []storage.PicklistValue{{Value: "Project", Active: true}}}
	if values, ok := plainDeveloperPicklist("Account", field); ok || values != nil {
		t.Fatal("overrode configured value set")
	}
	field.PicklistValuesConfigured = false
	values, ok := plainDeveloperPicklist("Account", field)
	if !ok || len(values) != 7 || values[0].Value != "Prospect" {
		t.Fatalf("captured values %v", values)
	}
	if values, ok := plainDeveloperPicklist("Opportunity", field); ok || !reflect.DeepEqual(values, []storage.PicklistValue(nil)) {
		t.Fatal("changed an unrelated field")
	}
}

func TestPlainDeveloperDescribeScope(t *testing.T) {
	org := storage.NewOrgState()
	org.Features = []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperDescribeShape}
	machine := New(nil)
	machine.SetOrg(&org)
	user := Object("User")
	profile := Object("Profile")
	profile.Fields["Name"] = String("System Administrator")
	user.Fields["Profile"] = profile
	user.Fields["LocaleSidKey"] = String("en_US")
	machine.executionUser = user
	machine.currentMethod = Method{APIVersion: "62.0"}
	if !machine.plainDeveloperDescribe("Account") {
		t.Fatal("captured context not selected")
	}
	machine.currentMethod.APIVersion = "63.0"
	if machine.plainDeveloperDescribe("Account") {
		t.Fatal("uncaptured API selected")
	}
	machine.currentMethod.APIVersion = "62.0"
	machine.executionUser.Fields["LocaleSidKey"] = String("fr_FR")
	if machine.plainDeveloperDescribe("Account") {
		t.Fatal("uncaptured locale selected")
	}
	machine.executionUser.Fields["LocaleSidKey"] = String("en_US")
	machine.executionUser.Fields["Profile"].Fields["Name"] = String("Standard User")
	if machine.plainDeveloperDescribe("Account") {
		t.Fatal("uncaptured profile selected")
	}
	machine.executionUser.Fields["Profile"].Fields["Name"] = String("System Administrator")
	org.Features = nil
	if machine.plainDeveloperDescribe("Account") {
		t.Fatal("unspecified shape selected")
	}
}
