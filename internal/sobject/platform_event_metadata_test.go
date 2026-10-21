package sobject

import (
	"testing"

	"github.com/glade-sh/glade/internal/schema"
)

// The CustomObject publishBehavior contract specifies PublishImmediately when
// omitted: https://developer.salesforce.com/docs/atlas.en-us.api_meta.meta/api_meta/customobject.htm
func TestBuildDescribeRegistryDefaultsPlatformEventPublishBehavior(t *testing.T) {
	for _, tc := range []struct {
		name, behavior, want string
	}{
		{"SignalLegacy__e", "", "PublishImmediately"},
		{"producer__SignalLegacy__e", "", "PublishImmediately"},
		{"SignalImmediate__e", "PublishImmediately", "PublishImmediately"},
		{"SignalDeferred__e", "PublishAfterCommit", "PublishAfterCommit"},
		{"SignalLog__c", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			registry := BuildDescribeRegistry(schema.Schema{Objects: []schema.Object{{
				Name: tc.name, PublishBehavior: tc.behavior,
			}}})
			definition := ToObjectDefinition(registry.Objects[tc.name])
			if got := definition.Metadata["publishBehavior"]; got != tc.want {
				t.Fatalf("publishBehavior = %q, want %q", got, tc.want)
			}
		})
	}
}
