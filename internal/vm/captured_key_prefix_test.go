package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestCapturedStandardKeyPrefixConstructor(t *testing.T) {
	captured, known := storage.StandardObjectDescribeKeyPrefix("MessagingSession")
	if !known || captured == nil {
		t.Fatal("missing captured stable prefix")
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(fmt.Sprintf(`
String prefix = MessagingSession.SObjectType.getDescribe().getKeyPrefix();
Id matching = Id.valueOf('%s000000000123');
MessagingSession row = new MessagingSession(Id=matching);
System.assertEquals(matching, row.Id);
System.assertEquals('%s', prefix);
Boolean rejected = false;
try { MessagingSession invalid = new MessagingSession(Id='001000000000123'); }
catch (TypeException e) { rejected = true; }
System.assert(rejected);
`, *captured, *captured), CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			storage.EnsureStandardObject(&org, "MessagingSession")
			machine := New(nil)
			machine.SetOrg(&org)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
