package vm

import (
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestJSONDeserializeConcreteSObjectEnvelope(t *testing.T) {
	program, err := CompileAnonymousWithOptions(`
String payload = '{"CronTrigger":{"id":"08e7A00000SlgHHQAZ","State":"WAITING","CronJobDetail":{"Name":"owned (selected)"}}}';
CronTrigger row = (CronTrigger)JSON.deserialize(payload, CronTrigger.class);
System.assertEquals('08e7A00000SlgHHQAZ', String.valueOf(row.Id));
System.assertEquals('WAITING', row.State);
System.assertEquals('owned (selected)', row.CronJobDetail.Name);
System.assertEquals('selected', row.CronJobDetail.Name.substringBetween('(', ')'));
Map<String,Object> untyped = (Map<String,Object>)JSON.deserializeUntyped(payload);
System.assertEquals(true, untyped.containsKey('CronTrigger'));
CronTrigger flat = (CronTrigger)JSON.deserialize('{"State":"WAITING","CronJobDetail":{"Name":"flat"}}', CronTrigger.class);
System.assertEquals('flat', flat.CronJobDetail.Name);
`, CompileOptions{APIVersion: "66.0"})
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "CronTrigger")
	storage.EnsureStandardObject(&org, "CronJobDetail")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestJSONSObjectEnvelopeRecognitionIsNarrow(t *testing.T) {
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "CronTrigger")
	machine.SetOrg(&org)
	for _, tc := range []struct {
		name, target string
		raw          any
	}{
		{"unrelated key", "CronTrigger", map[string]any{"Other": map[string]any{"State": "WAITING"}}},
		{"sibling fields", "CronTrigger", map[string]any{"CronTrigger": map[string]any{"State": "WAITING"}, "State": "DELETED"}},
		{"scalar envelope", "CronTrigger", map[string]any{"CronTrigger": "WAITING"}},
		{"generic object", "Object", map[string]any{"CronTrigger": map[string]any{"State": "WAITING"}}},
		{"list target", "List<CronTrigger>", []any{map[string]any{"CronTrigger": map[string]any{"State": "WAITING"}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := machine.jsonRootSObjectPayload(tc.target, tc.raw); !reflect.DeepEqual(got, tc.raw) {
				t.Fatalf("unproven envelope transformed: %#v", got)
			}
		})
	}
}
