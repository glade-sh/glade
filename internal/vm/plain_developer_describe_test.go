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
	user.Fields["LanguageLocaleKey"] = String("en_US")
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
	for _, version := range []string{"62.0", "67.0"} {
		machine.currentMethod.APIVersion = version
		machine.executionUser.Fields["LanguageLocaleKey"] = String("fr")
		if machine.plainDeveloperDescribe("Account") {
			t.Fatalf("uncaptured language selected at %s", version)
		}
		machine.executionUser.Fields["LanguageLocaleKey"] = String("en_US")
		if !machine.plainDeveloperDescribe("Account") {
			t.Fatalf("English context not restored at %s", version)
		}
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

func TestPlainDeveloperTokenDoesNotWarmOrdinaryFieldCache(t *testing.T) {
	for _, language := range []string{"en_US", "fr"} {
		t.Run(language, func(t *testing.T) {
			org := testDataOrg()
			org.Features = []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperDescribeShape}
			machine := New(nil)
			machine.SetOrg(&org)
			user := Object("User")
			profile := Object("Profile")
			profile.Fields["Name"] = String("System Administrator")
			user.Fields["Profile"] = profile
			user.Fields["LocaleSidKey"] = String("en_US")
			user.Fields["LanguageLocaleKey"] = String(language)
			machine.executionUser = user
			machine.currentMethod = Method{APIVersion: "62.0"}
			if _, err := machine.describeFieldValue("Account", "Tier"); err == nil {
				t.Fatal("cold ordinary lookup unexpectedly succeeded")
			}
			token := machine.sObjectFieldTokenFromField("Account", plainDeveloperTierField())
			token.Fields["__plainDeveloperTier"] = Bool(true)
			if _, _, _, _, err := machine.callSObjectFieldDescribe(token, nil, nil, false); err != nil {
				t.Fatal(err)
			}
			if _, err := machine.describeFieldValue("Account", "Tier"); err == nil {
				t.Fatal("token warmed ordinary field cache")
			}
		})
	}
}
