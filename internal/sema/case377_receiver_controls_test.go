package sema

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// These controls preserve existing named bindings and declared object metadata.
func TestCase377TriggerTokenKnownReceivers(t *testing.T) {
	for _, version := range []string{"62.0", "67.0"} {
		for _, tc := range []struct{ name, source, receiver string }{
			{"custom object", "Schema.SObjectType token = Garden__c.SObjectType;", "Garden__c"},
			{"declared class", "Schema.SObjectType token = TokenHolder.SObjectType;", "TokenHolder"},
			{"local binding", "TokenHolder localValue = new TokenHolder(); Schema.SObjectType token = localValue.SObjectType;", "localValue"},
		} {
			t.Run(version+"/"+tc.name, func(t *testing.T) {
				root := t.TempDir()
				trigger := filepath.Join(root, "T.trigger")
				class := filepath.Join(root, "TokenHolder.cls")
				writeSemaFile(t, trigger, "trigger T on Account (before insert) { "+tc.source+" }")
				writeSemaFile(t, class, "public class TokenHolder { public static Schema.SObjectType SObjectType; }")
				index := typesys.Build(project.Project{Root: root, SourceAPIVersion: version, ApexFiles: []string{trigger, class}}, schema.Schema{Objects: []schema.Object{{Name: "Account"}, {Name: "Garden__c"}}})
				result := Analyze(index)
				for _, d := range result.Diagnostics {
					if d.NativeMessage == "Variable does not exist: "+tc.receiver {
						t.Fatalf("known receiver rejected: %#v", d)
					}
				}
			})
		}
	}
}
