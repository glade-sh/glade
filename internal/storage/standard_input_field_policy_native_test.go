package storage

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"testing"
)

func TestStandardFieldPolicyNativeDMLInputSidecarRoute(t *testing.T) {
	stream, err := gzip.NewReader(bytes.NewReader(standardFieldPolicyData))
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	var policy struct {
		Objects map[string]struct {
			InputFields []string `json:"input_fields"`
		}
	}
	if err := json.NewDecoder(stream).Decode(&policy); err != nil {
		t.Fatal(err)
	}
	fields := policy.Objects["EmailMessage"].InputFields
	if len(fields) != 1 || fields[0] != "ToIds" {
		t.Fatalf("native DML input ToIds missing from sidecar: %v", fields)
	}
}

func TestStandardFieldPolicyNativeInsertInputCaseAndBoundaryRoutes(t *testing.T) {
	for _, route := range []struct {
		object, field string
		allowed       bool
	}{
		{"EmailMessage", "ToIds", true}, {"emailmessage", "toids", true}, {"EMAILMESSAGE", "TOIDS", true},
		{"Account", "ToIds", false}, {"EmailMessage", "UnknownOwnedField", false}, {"EmailMessage", "", false},
	} {
		t.Run(route.object+"/"+route.field, func(t *testing.T) {
			if got := StandardObjectInsertInputFieldAllowed(route.object, route.field); got != route.allowed {
				t.Fatalf("input allowed=%v, want %v", got, route.allowed)
			}
		})
	}
}
