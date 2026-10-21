package apextest

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func TestOrgFromIndexPreservesCapturedStandardKeyPrefix(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/MessagingSession/MessagingSession.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Messaging Session</label><pluralLabel>Messaging Sessions</pluralLabel></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/SessionIdentityProbe.cls"), `public class SessionIdentityProbe { public static MessagingSession copy(Id value) { return new MessagingSession(Id=value); } }`)
	index := loadTestIndex(t, root)
	org := orgFromIndex(index)
	captured, known := storage.StandardObjectDescribeKeyPrefix("MessagingSession")
	if !known || captured == nil {
		t.Fatal("missing captured stable prefix")
	}
	if got := org.Objects["MessagingSession"].Definition.KeyPrefix; got != *captured {
		t.Fatalf("project prefix = %q, captured %q", got, *captured)
	}
	program, err := vm.CompileAnonymousWithOptions(`Id matching=Id.valueOf(MessagingSession.SObjectType.getDescribe().getKeyPrefix()+'000000000123'); MessagingSession row=new MessagingSession(Id=matching); System.assertEquals(matching,row.Id);`, vm.CompileOptions{APIVersion: "67.0"})
	if err != nil {
		t.Fatal(err)
	}
	machine := vm.New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
