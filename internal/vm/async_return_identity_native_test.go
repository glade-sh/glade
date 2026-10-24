package vm

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestAsyncReturnIdentityAnonymousAndIsTestNative(t *testing.T) {
	dir := filepath.Join("..", "apextest", "testdata", "conformance", "async_return_identity")
	file, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 61 {
		t.Fatalf("incomplete native return identity matrix: %v", err)
	}
	for _, row := range rows[1:] {
		api, route, operation := row[0], row[1], row[2]
		if route == "named" {
			continue
		}
		t.Run(api+"/"+route+"/"+operation, func(t *testing.T) {
			machine := New(nil)
			org := storage.NewOrgState()
			org.APIVersion = api
			machine.SetOrg(&org)
			if route == "isTest" {
				machine.EnableTestContext()
			}
			for _, class := range []Class{
				{Name: "ReturnIdentityQueue", Interfaces: []string{"Queueable"}},
				{Name: "ReturnIdentityBatch", Interfaces: []string{"Database.Batchable<SObject>"}},
			} {
				if err := machine.RegisterClass(class); err != nil {
					t.Fatal(err)
				}
			}
			for _, typ := range []string{"Id", "String"} {
				body, err := CompileAnonymous("return '" + typ + "';")
				if err != nil {
					t.Fatal(err)
				}
				if err := machine.RegisterMethod(Method{Name: "AsyncReturnIdentityProbe.signature", ClassName: "AsyncReturnIdentityProbe", IsStatic: true, ReturnType: "String", Params: []Param{{Name: "value", Type: typ}}, Program: body}); err != nil {
					t.Fatal(err)
				}
			}
			for _, method := range []struct{ name, returns, body string }{
				{"stringReturn", "String", "return System.enqueueJob(new ReturnIdentityQueue());"},
				{"idReturn", "Id", "return System.enqueueJob(new ReturnIdentityQueue());"},
				{"objectReturn", "Object", "return System.enqueueJob(new ReturnIdentityQueue());"},
				{"stringLength", "Integer", "return value.length();"},
			} {
				body, err := CompileAnonymousWithOptions(method.body, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				target := Method{Name: "AsyncReturnIdentityProbe." + method.name, ClassName: "AsyncReturnIdentityProbe", IsStatic: true, ReturnType: method.returns, Program: body, APIVersion: api}
				if method.name == "stringLength" {
					target.Params = []Param{{Name: "value", Type: "String"}}
				}
				if err := machine.RegisterMethod(target); err != nil {
					t.Fatal(err)
				}
			}
			source, err := os.ReadFile(filepath.Join(dir, operation+".apex"))
			if err != nil {
				t.Fatal(err)
			}
			program, err := CompileAnonymousWithOptions(string(source), CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			result, err := machine.Execute(program)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Debug) != 1 || !strings.HasPrefix(result.Debug[0], "OBS|") {
				t.Fatalf("missing observation: %v", result.Debug)
			}
			var got, want map[string]any
			if json.Unmarshal([]byte(strings.TrimPrefix(result.Debug[0], "OBS|")), &got) != nil || json.Unmarshal([]byte(row[3]), &want) != nil {
				t.Fatal("invalid native return identity observation")
			}
			for binding, field := range map[string]string{"returned": "rawLength", "submitted": "typedValueOfLength"} {
				if value, ok := machine.Globals[binding]; ok {
					text, err := stringStatic("String.valueOf", []Value{value})
					if err != nil || float64(len(text.Text)) != want[field] {
						t.Errorf("stdlib String.valueOf(%s) length=%d, native=%v (%v)", binding, len(text.Text), want[field], err)
					}
				}
			}
			if submitted, ok := machine.Globals["submitted"]; ok {
				text, err := evalBinary("+", String(""), submitted)
				if err != nil || float64(len(text.Text)) != want["typedConcatLength"] {
					t.Errorf("free evaluator concatenation length=%d, native=%v (%v)", len(text.Text), want["typedConcatLength"], err)
				}
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native=%v; Glade=%v", want, got)
			}
		})
	}
}
