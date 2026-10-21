package sema

import (
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/schema"
)

func TestScheduledReferenceMetadataSupportsIdOperands(t *testing.T) {
	for _, tc := range []struct {
		name  string
		query string
		valid bool
	}{
		{"reference fallback", "SELECT Id FROM CronTrigger WHERE CronJobDetailId IN (SELECT Id FROM CronJobDetail)", true},
		{"reference control", "SELECT Id FROM CronTrigger WHERE OwnerId IN (SELECT Id FROM User)", true},
		{"primary key control", "SELECT Id FROM CronTrigger WHERE Id IN (SELECT CronTriggerId FROM AsyncApexJob)", true},
		{"text control", "SELECT Id FROM CronTrigger WHERE CronExpression IN (SELECT Id FROM CronJobDetail)", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := analyzeQueryProbe(t, "public class ScheduledReferenceProbe { public void run() { CronTrigger record = new CronTrigger(); Id detailId = record.CronJobDetailId; List<CronTrigger> rows = ["+tc.query+"]; } }", schema.Schema{Objects: []schema.Object{{Name: "CronTrigger"}}})
			if result.HasErrors() == tc.valid {
				t.Fatalf("valid=%v, diagnostics: %#v", tc.valid, result.Diagnostics)
			}
			if !tc.valid && !hasDiagnosticCode(result.Diagnostics, "GLADESEMA_QUERY_CONTRACT") {
				t.Fatalf("text operand lost its query contract diagnostic: %#v", result.Diagnostics)
			}
		})
	}
}

func TestStandardReferenceMetadataCanonicalizesScalarIdFallback(t *testing.T) {
	// The owned standard describe catalog captures CronJobDetailId as a
	// reference to CronJobDetail, rather than a scalar ID field.
	for _, tc := range []struct {
		name, fieldName, declaredType, wantType string
		wantTargets                             []string
	}{
		{"fallback", "CronJobDetailId", "", "Reference", []string{"CronJobDetail"}},
		{"scalar id", "CronJobDetailId", "Id", "Reference", []string{"CronJobDetail"}},
		{"explicit lookup", "CronJobDetailId", "Lookup", "Lookup", []string{"CronJobDetail"}},
		{"explicit text", "CronJobDetailId", "Text", "Text", []string{"CronJobDetail"}},
		{"primary key", "Id", "Id", "Id", nil},
		{"nonreference id", "CronExpression", "Id", "Id", nil},
		{"custom id", "LocalKey__c", "Id", "Id", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			object := schema.Object{Name: "CronTrigger"}
			if tc.declaredType != "" {
				object.Fields = []schema.Field{{Name: tc.fieldName, Type: tc.declaredType}}
			}
			provider := newSemaSObjectFieldProvider("", object)
			field, ok := provider.lookup(tc.fieldName)
			if !ok || !strings.EqualFold(field.Type, tc.wantType) || !reflect.DeepEqual(field.ReferenceTo, tc.wantTargets) {
				t.Fatalf("field=%#v, want type %s targets %v", field, tc.wantType, tc.wantTargets)
			}
			if len(object.Fields) > 0 && object.Fields[0].Type != tc.declaredType {
				t.Fatal("lookup mutated the supplied schema")
			}
			field.ReferenceTo = append(field.ReferenceTo[:0], "ChangedTarget")
			again, _ := provider.lookup(tc.fieldName)
			if !reflect.DeepEqual(again.ReferenceTo, tc.wantTargets) {
				t.Fatalf("returned field mutation changed the cached targets: %#v", again)
			}
		})
	}
}
