package resource

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestDuplicateRuleMetadataInputsRootDependencyAndShimRoutes(t *testing.T) {
	for _, route := range []string{"root", "dependency", "shim"} {
		t.Run(route, func(t *testing.T) {
			child := map[string]any{"matchingRuleFiles": []string{"Contact.matchingRule-meta.xml"}, "duplicateRuleFiles": []string{"Contact.Grove.duplicateRule-meta.xml"}}
			body := child
			if route != "root" {
				field := map[string]string{"dependency": "managedPackageDependencies", "shim": "packageShims"}[route]
				body = map[string]any{field: []any{map[string]any{"status": "loaded", "project": child}}}
			}
			data, err := json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
			var p project.Project
			if err := json.Unmarshal(data, &p); err != nil {
				t.Fatal(err)
			}
			want := []string{"Contact.Grove.duplicateRule-meta.xml", "Contact.matchingRule-meta.xml"}
			if got := SetupRecordMetadataFiles(p); !reflect.DeepEqual(got, want) {
				t.Fatalf("metadata inputs=%v want %v", got, want)
			}
		})
	}
}
