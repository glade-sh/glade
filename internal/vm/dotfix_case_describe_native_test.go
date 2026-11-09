package vm

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestDotfixNativeCaseFieldDescribe(t *testing.T) {
	for _, api := range []string{"62", "67"} {
		t.Run(api, func(t *testing.T) {
			data, err := os.ReadFile("../apextest/testdata/conformance/dotfix_fls_followups/native-minimal-permissions-" + api + ".json")
			if err != nil {
				t.Fatal(err)
			}
			var native struct {
				Describe map[string]struct {
					Fields map[string]struct {
						Permissionable bool `json:"permissionable"`
						Accessible     bool `json:"accessible"`
						Createable     bool `json:"createable"`
						Updateable     bool `json:"updateable"`
					} `json:"fields"`
				} `json:"describe"`
			}
			if err := json.Unmarshal(data, &native); err != nil {
				t.Fatal(err)
			}
			machine := dotfixRestrictedMachine(api + ".0")
			machine.currentMethod.APIVersion = api + ".0"
			for _, name := range []string{"OwnerId", "Status", "Origin", "Subject"} {
				want, ok := native.Describe["Case"].Fields[name]
				if !ok {
					t.Fatalf("native field %s missing", name)
				}
				_, field, ok := machine.sObjectFieldDefinition("Case", name)
				if !ok {
					t.Fatalf("Case.%s not defined", name)
				}
				if got := storage.FieldFlagValue(field.Permissionable, true); got != want.Permissionable {
					t.Errorf("%s permissionable=%t native=%t", name, got, want.Permissionable)
				}
				for method, expected := range map[string]bool{"isAccessible": want.Accessible, "isCreateable": want.Createable, "isUpdateable": want.Updateable} {
					if got := machine.currentUserFieldPermission("Case", name, method); got != expected {
						t.Errorf("%s %s=%t native=%t", name, method, got, expected)
					}
				}
			}
		})
	}
}
