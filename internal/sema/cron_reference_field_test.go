package sema

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// The Object Reference documents CronJobDetailId as a lookup to CronJobDetail:
// https://developer.salesforce.com/docs/atlas.en-us.object_reference.meta/object_reference/sforce_api_objects_crontrigger.htm
func TestCronTriggerJobDetailReferenceMetadata(t *testing.T) {
	for _, partial := range []bool{false, true} {
		t.Run(fmt.Sprintf("partial=%t", partial), func(t *testing.T) {
			provider := newSemaSObjectFieldProvider("", schema.Object{Name: "CronTrigger", Partial: partial})
			field, ok := provider.lookup("CronJobDetailId")
			if !ok || !(strings.EqualFold(field.Type, "Lookup") || strings.EqualFold(field.Type, "Reference")) || len(field.ReferenceTo) != 1 || field.ReferenceTo[0] != "CronJobDetail" || field.RelationshipName != "CronJobDetail" {
				t.Fatalf("job detail lookup metadata = %#v, found %t", field, ok)
			}
			id, ok := provider.lookup("Id")
			if !ok || !strings.EqualFold(id.Type, "Id") {
				t.Fatalf("record identity metadata = %#v, found %t", id, ok)
			}
		})
	}
}

func TestCronTriggerJobDetailSemiJoinUsesReferenceMetadata(t *testing.T) {
	for _, api := range []string{"61.0", "62.0", "67.0", "68.0"} {
		for _, op := range []string{"IN", "NOT IN"} {
			t.Run(api+"/"+op, func(t *testing.T) {
				source := fmt.Sprintf(`public class JobQueryProbe {
  public static void run() {
    List<CronTrigger> jobs = [SELECT Id FROM CronTrigger
      WHERE State = 'WAITING' AND CronJobDetailId %s
        (SELECT Id FROM CronJobDetail WHERE Name = 'Scheduled probe')];
  }
}`, op)
				for name, objects := range map[string][]schema.Object{
					"catalog":         nil,
					"standard schema": {{Name: "CronTrigger"}, {Name: "CronJobDetail"}},
					"partial schema":  {{Name: "CronTrigger", Partial: true}, {Name: "CronJobDetail", Partial: true}},
				} {
					t.Run(name, func(t *testing.T) {
						result := analyzeQueryProbeWithProject(t, source, typesys.ProjectInfo{SourceAPIVersion: api}, schema.Schema{Objects: objects})
						if result.HasErrors() {
							t.Fatalf("documented reference operand rejected: %#v", result.Diagnostics)
						}
					})
				}
			})
		}
	}
}

func TestCronTriggerSemiJoinStillRejectsNonReferenceFields(t *testing.T) {
	for _, api := range []string{"61.0", "62.0", "67.0", "68.0"} {
		for _, field := range []string{"CronExpression", "TimesTriggered"} {
			t.Run(api+"/"+field, func(t *testing.T) {
				source := fmt.Sprintf(`public class JobQueryProbe {
  public static void run() {
    List<CronTrigger> jobs = [SELECT Id FROM CronTrigger WHERE %s IN (SELECT Id FROM CronJobDetail)];
  }
}`, field)
				result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"JobQueryProbe.cls": source}, api)
				found := false
				for _, item := range result.Diagnostics {
					if item.Code == "GLADESEMA_QUERY_CONTRACT" && strings.Contains(item.Message, "cannot use: '"+field+"'") {
						found = true
					}
				}
				if !found {
					t.Fatalf("non-reference field accepted: %#v", result.Diagnostics)
				}
			})
		}
	}
}

func TestSemiJoinTextFieldNameDoesNotConferReferenceType(t *testing.T) {
	result := analyzeQueryProbe(t, `public class JobQueryProbe {
      public static void run() {
        List<Probe__c> rows = [SELECT Id FROM Probe__c WHERE TrackingId__c IN (SELECT Id FROM Account)];
      }
    }`, schema.Schema{Objects: []schema.Object{{Name: "Probe__c", Fields: []schema.Field{{Name: "TrackingId__c", Type: "Text"}}}}})
	if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA_QUERY_CONTRACT") {
		t.Fatalf("text field accepted as a reference: %#v", result.Diagnostics)
	}
}
