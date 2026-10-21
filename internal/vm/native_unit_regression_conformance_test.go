package vm

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

type nativeUnitRegressionRow struct {
	api, name string
	want      map[string]any
}

func TestNativeUnitRegressionRows(t *testing.T) {
	rows := readNativeUnitRegressionRows(t)
	if len(rows) != 12 {
		t.Fatalf("native rows=%d, want 12", len(rows))
	}
	for _, row := range rows {
		row := row
		t.Run(row.api+"/"+row.name, func(t *testing.T) {
			switch row.name {
			case "direct_edit_share":
				got := runNativeEditShare(t, row.api)
				if !reflect.DeepEqual(got, row.want) {
					t.Fatalf("native=%v local=%v", row.want, got)
				}
			case "json_loose_date":
				program, err := CompileAnonymousWithOptions(
					`Date value = (Date)JSON.deserialize('"2024-2-29 00:00:00"', Date.class);`,
					CompileOptions{APIVersion: row.api},
				)
				if err != nil {
					t.Fatal(err)
				}
				_, err = New(nil).Execute(program)
				var runtimeErr *RuntimeError
				if !errors.As(err, &runtimeErr) {
					t.Fatalf("local error=%v, want native JSON exception", err)
				}
				localType := runtimeErr.Type
				if !strings.Contains(localType, ".") {
					localType = "System." + localType
				}
				if localType != row.want["errorType"] {
					t.Fatalf("local type=%q native type=%q", localType, row.want["errorType"])
				}
				if runtimeErr.Message != row.want["errorMessage"] {
					t.Fatalf("local message=%q native message=%q", runtimeErr.Message, row.want["errorMessage"])
				}
			case "stub_static_factory":
				if row.want["value"] != "mock" {
					t.Fatalf("native row=%v", row.want)
				}
				TestExecTestCreateStubInterceptsStaticFactoryChain(t)
			case "stub_static_factory_async":
				if !reflect.DeepEqual(row.want["names"], []any{"mock"}) {
					t.Fatalf("native row=%v", row.want)
				}
				TestExecTestCreateStubStaticFactorySurvivesStopTestAsync(t)
			case "stub_map_values_list_sobject":
				if row.want["callbackCount"] != float64(1) || row.want["argumentType"] != "List<SObject>" {
					t.Fatalf("native row=%v", row.want)
				}
				TestExecTestCreateStubStaticFactoryInterceptsMapValuesListSObject(t)
			case "stub_platform_type_rejection":
				if row.want["callbackCount"] != float64(0) || row.want["errorType"] != "System.TypeException" {
					t.Fatalf("native row=%v", row.want)
				}
				t.Run("member-dispatch", TestExecTestCreateStubRejectsSObjectBeforeMemberDispatch)
				t.Run("dynamic-dispatch", TestExecTestCreateStubRejectsSObjectBeforeDynamicDispatch)
			default:
				t.Fatalf("unexpected native row %q", row.name)
			}
		})
	}
}

func readNativeUnitRegressionRows(t *testing.T) []nativeUnitRegressionRow {
	t.Helper()
	file, err := os.Open("testdata/unit_regressions/native_capture/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.FieldsPerRecord = 3
	reader.LazyQuotes = true
	header, err := reader.Read()
	if err != nil || !reflect.DeepEqual(header, []string{"source_api", "case", "observation"}) {
		t.Fatalf("native row header=%v error=%v", header, err)
	}
	var rows []nativeUnitRegressionRow
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		var want map[string]any
		if err := json.Unmarshal([]byte(record[2]), &want); err != nil {
			t.Fatalf("invalid native row %s/%s: %v", record[0], record[1], err)
		}
		rows = append(rows, nativeUnitRegressionRow{api: record[0], name: record[1], want: want})
	}
	return rows
}

func runNativeEditShare(t *testing.T, api string) map[string]any {
	t.Helper()
	machine := recordSharingControlMachine(t, "Private")
	org := machine.Org
	state := org.Objects["Widget__c"]
	target := state.Records["a00000000000001"]
	org.Objects["Widget__Share"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Widget__Share"},
		Records: map[storage.ID]storage.Record{
			"a01000000000001": {
				ID: "a01000000000001", Object: "Widget__Share",
				Fields: map[string]storage.Value{
					"ParentId":      storage.IDValue(target.ID),
					"UserOrGroupId": storage.IDValue("005000000000002"),
					"AccessLevel":   storage.StringValue("Edit"),
					"RowCause":      storage.StringValue("Manual"),
				},
			},
		},
	}
	if err := machine.RegisterClass(Class{Name: "ShareWriter", APIVersion: api, Modifiers: []string{"with sharing"}}); err != nil {
		t.Fatal(err)
	}
	body, err := CompileAnonymousWithOptions(
		"Widget__c row=new Widget__c(Id='a00000000000001',Name='changed');update row;",
		CompileOptions{APIVersion: api},
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = machine.callMethod(Method{
		Name: "ShareWriter.write", ClassName: "ShareWriter", IsStatic: true,
		APIVersion: api, SourceContextBound: true, SharingMode: "with sharing", Program: body,
	}, nil, &Result{})
	visible := machine.userModeRecordVisible("Widget__c", target, "005000000000002")
	visibleCount := float64(0)
	if visible {
		visibleCount = 1
	}
	return map[string]any{
		"visibleCount":    visibleCount,
		"updateSucceeded": err == nil,
		"storedName":      org.Objects["Widget__c"].Records[target.ID].Fields["Name"].String,
	}
}
