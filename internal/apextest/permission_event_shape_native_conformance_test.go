package apextest

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestPermissionEventShapeNativeConformance(t *testing.T) {
	runStandardEventNativeRows(t, "permission_event_shape")
}

func TestPermissionEventDescribeNativeConformance(t *testing.T) {
	for _, api := range []string{"62", "67"} {
		t.Run(api, func(t *testing.T) {
			raw, err := os.ReadFile("testdata/conformance/permission_event_shape/api" + api + "/describe.json")
			if err != nil {
				t.Fatal(err)
			}
			var captured struct {
				Name   string
				Fields []struct{ Name, Type string }
			}
			if err = json.Unmarshal(raw, &captured); err != nil {
				t.Fatal(err)
			}
			definition, ok := storage.StandardObjectDefinition(captured.Name)
			if !ok {
				t.Fatal("native type missing from shared schema")
			}
			for _, field := range captured.Fields {
				if _, ok := storage.ResolveFieldName(definition, "", field.Name); !ok {
					t.Errorf("native field missing: %s", field.Name)
				}
			}
		})
	}
}
