package vm

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The project capture test replays these same rows through anonymous execution,
// direct test entry and the project runner. This isolates transaction checking.
func TestMixedDMLNativeContextRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		f, err := os.Open(filepath.Join("testdata", "conformance", "mixed_dml_contexts", "native", "api"+strings.TrimSuffix(api, ".0"), "org.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		reader := csv.NewReader(f)
		reader.Comma = '\t'
		rows, err := reader.ReadAll()
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range rows[1:] {
			t.Run(api+"/"+row[1]+"/"+row[2], func(t *testing.T) {
				var want string
				if err := json.Unmarshal([]byte(row[4]), &want); err != nil {
					t.Fatal(err)
				}
				machine := New(nil)
				machine.currentMethod = Method{APIVersion: api}
				if row[1] == "istest" {
					machine.EnableTestContext()
				}
				if row[2] == "startbothrole" {
					if _, err := machine.testStart(); err != nil {
						t.Fatal(err)
					}
				}
				if row[2] == "runasrole" {
					machine.testContext.RunAsDepth = 1
				}
				if err := machine.checkMixedDML("insert", []storage.Record{{Object: "Account"}}); err != nil {
					t.Fatal(err)
				}
				if row[2] == "startrole" {
					if _, err := machine.testStart(); err != nil {
						t.Fatal(err)
					}
				}
				actor := storage.Record{Object: "User", Fields: map[string]storage.Value{}}
				if !strings.Contains(row[2], "no_role") && !strings.Contains(row[2], "norole") {
					actor.Fields["UserRoleId"] = storage.StringValue("00E000000000001")
				}
				got := "success"
				if err := machine.checkMixedDML("insert", []storage.Record{actor}); err != nil {
					thrown, ok := err.(*apexThrowError)
					if !ok {
						t.Fatal(err)
					}
					status, err := dmlExceptionDetail(thrown.value, "getDmlType", []Value{Int(0)})
					if err != nil || status.Fields["statusCode"].String() != "MIXED_DML_OPERATION" {
						t.Fatalf("mixed insert lost its DML status: %v, %v", status, err)
					}
					got = "System." + thrown.value.Type + ": " + thrown.value.Fields["message"].String()
				}
				if got != want {
					t.Fatalf("native=%s; Glade=%s", want, got)
				}
				if row[1] == "anonymous" && machine.testContext != nil {
					t.Fatal("anonymous DML checking enabled test-only APIs")
				}
			})
		}
	}
}
