package sema

import (
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
	"testing"
)

// Salesforce org.tsv q3share:2-9 captures valid and missing reason members.
func TestQ3StandardShareReasonCompileBoundary(t *testing.T) {
	for _, object := range []string{"AccountShare"} {
		t.Run(object, func(t *testing.T) {
			members := typeMembers{name: object, sobject: true}
			model := newSemaTypeMemberStateWithPlatform(&semaTypeMemberModel{members: map[string]typeMembers{normalizeName(object): members}}, nil).view()
			analyzer := NewAnalyzer()
			scope := newIRSemaScope(nil)
			if got := analyzer.securityAccessFieldMessage("Schema."+object+".RowCause.Manual", scope, model); got != "" {
				t.Fatalf("valid reason: %s", got)
			}
			if got := analyzer.securityAccessFieldMessage("Schema."+object+".RowCause.NotARealReason", scope, model); got != "Variable does not exist: NotARealReason" {
				t.Fatalf("missing reason diagnostic: %q", got)
			}
		})
	}
}

func TestQ3SharingReasonAnonymousCompileRejections(t *testing.T) {
	index := typesys.Index{Objects: []schema.Object{{Name: "AccountShare", Partial: true}}}
	for _, api := range []string{"62.0", "67.0"} {
		for _, body := range []string{
			"System.debug(Schema.AccountShare.RowCause.NotARealReason);",
			"Schema.SObjectField token = Schema.AccountShare.RowCause; System.debug(token.Manual);",
			"Schema.SObjectField token = Schema.AccountShare.RowCause; System.debug(token.Owner);",
			"Schema.SObjectField token = Schema.AccountShare.RowCause; System.debug(token.Name);",
			"Schema.SObjectField token = Schema.AccountShare.RowCause; System.debug(token.NotARealMember);",
		} {
			t.Run(api+"/"+body, func(t *testing.T) {
				result := AnalyzeAnonymous(index, body, api)
				if !result.HasErrors() {
					t.Fatalf("native rejected but analysis accepted: %v", result.Diagnostics)
				}
			})
		}
	}
	valid := "Schema.SObjectField token = Schema.AccountShare.RowCause; System.assertEquals('RowCause',token.getDescribe().getName());"
	for _, api := range []string{"62.0", "67.0"} {
		if result := AnalyzeAnonymous(index, valid, api); result.HasErrors() {
			t.Fatalf("valid method rejected: %v", result.Diagnostics)
		}
	}

}
