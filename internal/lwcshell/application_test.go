package lwcshell

import (
	"reflect"
	"testing"
)

func TestLoadCustomApplicationParsesNavigation(t *testing.T) {
	path := writeTempFile(t, "Credentialing_Console.app-meta.xml", `<CustomApplication xmlns="http://soap.sforce.com/2006/04/metadata">
  <label>Credentialing Console</label>
  <navType>Console</navType>
  <tabs>standard-home</tabs>
  <tabs>standard-Account</tabs>
  <tabs>Credentialing_Workflow__c</tabs>
</CustomApplication>`)

	app, err := LoadCustomApplication(path)
	if err != nil {
		t.Fatal(err)
	}
	if app.Name != "Credentialing_Console" || app.Label != "Credentialing Console" {
		t.Fatalf("app header = %#v", app)
	}
	if !app.Console {
		t.Fatalf("Console = false, want true")
	}
	if app.DefaultLandingTab != "standard-home" {
		t.Fatalf("DefaultLandingTab = %q, want standard-home", app.DefaultLandingTab)
	}
	want := []string{"standard-home", "standard-Account", "Credentialing_Workflow__c"}
	if !reflect.DeepEqual(app.NavItems, want) {
		t.Fatalf("NavItems = %#v, want %#v", app.NavItems, want)
	}
}
