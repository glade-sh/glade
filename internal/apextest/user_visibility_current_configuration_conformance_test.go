package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

type userVisibilityConfiguration struct {
	Profiles map[string]struct {
		Name     string          `json:"name"`
		UserType string          `json:"userType"`
		License  string          `json:"license"`
		Grants   map[string]bool `json:"grants"`
	} `json:"profiles"`
	Sharing []struct {
		Object   string `json:"QualifiedApiName"`
		Internal string `json:"InternalSharingModel"`
		External string `json:"ExternalSharingModel"`
	} `json:"sharing"`
}

// These named observations cover only the captured internal Read configuration.
// Project the observed metadata into this fixture, never into runtime defaults.
// Private OWD, isolated grants and external actors remain explicit data carries.
func TestUserVisibilityCurrentConfigurationOrgConformance(t *testing.T) {
	const directory = "testdata/conformance/user_visibility_current_configuration"
	var digests map[string]string
	read := func(name string, target any) {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, target); err != nil {
			t.Fatal(err)
		}
	}
	read("digests.json", &digests)
	if len(digests) != 3 {
		t.Fatal("incomplete native artifact digests")
	}
	for name, digest := range digests {
		raw, err := os.ReadFile(filepath.Join(directory, name))
		hash := sha256.Sum256(raw)
		if err != nil || hex.EncodeToString(hash[:]) != digest {
			t.Fatalf("changed native artifact %s: %v", name, err)
		}
	}
	var data struct {
		ObservationCount int                         `json:"observationCount"`
		Configuration    userVisibilityConfiguration `json:"configuration"`
		TransportColumns []string                    `json:"transportColumns"`
		ProjectSources   map[string]struct {
			Source string `json:"source"`
			Digest string `json:"sha256"`
		} `json:"projectSources"`
		Rows []struct {
			ID        string `json:"id"`
			CallerAPI string `json:"callerAPI"`
			HelperAPI string `json:"helperAPI"`
			Class     string `json:"class"`
			Method    string `json:"method"`
			Native    struct {
				Value map[string]any `json:"value"`
			} `json:"native"`
		} `json:"rows"`
		Carries []struct {
			Shape  string `json:"shape"`
			Reason string `json:"reason"`
		} `json:"carries"`
	}
	read("user_visibility_current_configuration.json", &data)
	if data.ObservationCount != 28 || len(data.Rows) != 28 || len(data.ProjectSources) != 39 || len(data.TransportColumns) != 41 || len(data.Carries) != 5 {
		t.Fatal("incomplete native observation packet")
	}
	for _, carry := range data.Carries {
		if carry.Shape == "" || carry.Reason == "" {
			t.Fatal("missing native boundary")
		}
		t.Logf("unasserted boundary %s: %s", carry.Shape, carry.Reason)
	}
	root := t.TempDir()
	for path, source := range data.ProjectSources {
		hash := sha256.Sum256([]byte(source.Source))
		if source.Source == "" || hex.EncodeToString(hash[:]) != source.Digest {
			t.Fatalf("captured source changed: %s", path)
		}
		writeFile(t, filepath.Join(root, path), source.Source)
	}
	index := loadTestIndex(t, root)
	analysis := sema.Analyze(index)
	if index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("native accepts; Glade diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
	}
	org := userVisibilityCapturedOrg(t, index, data.Configuration)
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
	seen := map[string]bool{}
	exact := 0
	t.Cleanup(func() {
		t.Logf("UserVisibilityCurrentConfiguration: exact=%d", exact)
	})
	for _, row := range data.Rows {
		if row.ID == "" || seen[row.ID] || len(row.Native.Value) != len(data.TransportColumns) || row.Native.Value["captureId"] != row.ID || row.Native.Value["callerAPI"] != row.CallerAPI || row.Native.Value["helperAPI"] != row.HelperAPI {
			t.Fatalf("invalid native cell: %s", row.ID)
		}
		seen[row.ID] = true
		t.Run(row.ID, func(t *testing.T) {
			cases := Discover(index, Options{SelectedClasses: []string{row.Class}, SelectedMethod: row.Method})
			if len(cases) != 1 {
				t.Fatalf("missing captured method: %s.%s", row.Class, row.Method)
			}
			methods, methodErrors := compileTestMethodsForIndex(&index, cases)
			programs, programErrors := compileTestInvokePrograms(cases)
			key := testCaseKey(cases[0])
			if methodErrors[key] != nil || programErrors[key] != nil {
				t.Fatalf("named compilation: %v %v", methodErrors[key], programErrors[key])
			}
			machine := runner.newMachine()
			if err := machine.RegisterMethod(methods[key]); err != nil {
				t.Fatal(err)
			}
			machine.EnableTestContext()
			_, execErr := machine.ExecuteInClass(programs[key], row.Class)
			var fault *vm.RuntimeError
			if !errors.As(execErr, &fault) || fault.Type != "System.AssertException" {
				t.Fatalf("missing native observation assertion: %v", execErr)
			}
			_, payload, ok := strings.Cut(fault.ExceptionMessage(), "Expected: NATIVE_OBSERVATION, Actual: ")
			var frame []any
			if !ok || json.Unmarshal([]byte(payload), &frame) != nil || len(frame) != len(data.TransportColumns) {
				t.Fatalf("incomplete observation: %s", fault.ExceptionMessage())
			}
			got := map[string]any{}
			for i, column := range data.TransportColumns {
				got[column] = frame[i]
			}
			want := row.Native.Value
			if !reflect.DeepEqual(got, want) {
				wantJSON, _ := json.Marshal(want)
				gotJSON, _ := json.Marshal(got)
				t.Fatalf("native=%s; Glade=%s", wantJSON, gotJSON)
			}
			exact++
		})
	}
}

func userVisibilityCapturedOrg(t *testing.T, index typesys.Index, configuration userVisibilityConfiguration) storage.OrgState {
	t.Helper()
	org := orgFromIndex(index)
	org.APIVersion = "68.0" // Metadata transport version; class versions stay exact.
	for _, sharing := range configuration.Sharing {
		object, ok := org.Objects[sharing.Object]
		if !ok {
			t.Fatalf("missing captured object %s", sharing.Object)
		}
		object.Definition.SharingModel = sharing.Internal
		org.Objects[sharing.Object] = object
		// ExternalSharingModel is retained as provenance, not an external-actor test.
	}
	profiles := org.Objects["Profile"]
	permissions := org.Objects["PermissionSet"]
	licenses := org.Objects["UserLicense"]
	for _, snapshot := range configuration.Profiles {
		matched := 0
		for id, profile := range profiles.Records {
			if profile.Fields["Name"].String != snapshot.Name {
				continue
			}
			matched++
			profile.Fields["UserType"] = storage.StringValue(snapshot.UserType)
			for name, granted := range snapshot.Grants {
				profile.Fields[name] = storage.BooleanValue(granted)
			}
			profiles.Records[id] = profile
			licenseID := profile.Fields["UserLicenseId"].ID
			license, exists := licenses.Records[licenseID]
			if licenseID == "" || !exists || license.Fields == nil {
				t.Fatalf("missing captured profile license: %s", snapshot.Name)
			}
			license.Fields["LicenseDefinitionKey"] = storage.StringValue(snapshot.License)
			licenses.Records[licenseID] = license
			owned := 0
			for permissionID, permission := range permissions.Records {
				if permission.Fields["IsOwnedByProfile"].Boolean && permission.Fields["ProfileId"].ID == id {
					owned++
					for name, granted := range snapshot.Grants {
						permission.Fields[name] = storage.BooleanValue(granted)
					}
					permissions.Records[permissionID] = permission
				}
			}
			if owned != 1 {
				t.Fatalf("profile %s has %d owned PermissionSets", snapshot.Name, owned)
			}
		}
		if matched != 1 {
			t.Fatalf("profile fixture %s matched %d records", snapshot.Name, matched)
		}
	}
	org.Objects["Profile"] = profiles
	org.Objects["PermissionSet"] = permissions
	org.Objects["UserLicense"] = licenses
	return org
}
