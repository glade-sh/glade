package vm

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestSetupObjectSecurityEnforcedAPI62NativeVMRoutes(t *testing.T) {
	file, err := os.Open("../apextest/testdata/conformance/setup_object_read/security-enforced-api62.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma, reader.LazyQuotes = '\t', true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 2 {
		t.Fatalf("native security matrix: %v rows=%d", err, len(rows))
	}
	var native map[string]any
	if err := json.Unmarshal([]byte(rows[1][2]), &native); err != nil {
		t.Fatal(err)
	}
	for _, actor := range []string{"admin", "Standard User", "Minimum Access - Salesforce"} {
		for _, route := range []string{"inlineCount", "dynamicCount"} {
			t.Run("62.0/withSharing/SECURITY_ENFORCED/"+actor+"/"+route, func(t *testing.T) {
				org := storage.NewOrgState()
				storage.EnsureDeterministicPlatformData(&org)
				roles := org.Objects["UserRole"]
				for i := 1; i <= 3; i++ {
					id := storage.ID(fmt.Sprintf("00E00000000009%d", i))
					roles.Records[id] = storage.Record{ID: id, Object: "UserRole", System: storage.SystemFields{OwnerID: "005000000000001"}, Fields: map[string]storage.Value{"Name": storage.StringValue("Meadow Security")}}
				}
				org.Objects["UserRole"] = roles
				machine := New(nil)
				machine.SetOrg(&org)
				user, profile := Object("User"), Object("Profile")
				user.Fields["Id"] = String("005000000000099")
				profileName := actor
				if actor == "admin" {
					profileName = "System Administrator"
				}
				profile.Fields["Name"] = String(profileName)
				user.Fields["Profile"] = profile
				machine.executionUser = user
				query := "SELECT COUNT() FROM UserRole WHERE Name = 'Meadow Security' WITH SECURITY_ENFORCED"
				body := "return [" + query + "];"
				if route == "dynamicCount" {
					body = "return Database.countQuery('" + strings.ReplaceAll(query, "'", "\\'") + "');"
				}
				body = "try {" + body + "} catch (Exception e) { return 'denied'; }"
				program, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: "62.0"})
				if err != nil {
					t.Fatal(err)
				}
				value, err := machine.callMethod(Method{Name: "MeadowSecurityReader.read", ClassName: "MeadowSecurityReader", IsStatic: true, APIVersion: "62.0", SourceContextBound: true, SharingMode: "with sharing", Program: program}, nil, &Result{})
				expected := native[actor+".roles.security"]
				if count, ok := expected.(float64); ok {
					if err != nil || value.Kind != ValueInt || float64(value.Int) != count {
						t.Fatalf("native=%v; Glade=%+v error=%v", expected, value, err)
					}
				} else {
					if expected != "System.QueryException" || err != nil || value.Kind != ValueString || value.String() != "denied" {
						t.Fatalf("native denial=%v; Glade=%+v error=%v", expected, value, err)
					}
				}
			})
		}
	}
}
