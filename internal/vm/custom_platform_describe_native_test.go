package vm

import (
	"encoding/csv"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func customPlatformDescribeNativeRows(t *testing.T) [][]string {
	t.Helper()
	f, err := os.Open("../apextest/testdata/conformance/custom_platform_describe/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	return rows[1:]
}

func TestCustomPlatformDescribeNativeRowsLoadedCatalog(t *testing.T) {
	rows := customPlatformDescribeNativeRows(t)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api+"/describeGetter", func(t *testing.T) {
			org := storage.NewOrgState()
			org.APIVersion = api
			org.Objects["LinkProbe__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "LinkProbe__c"}}
			var want []string
			for _, row := range rows {
				if row[0] != api || row[1] != "anonymous" || row[2] != "describe" {
					continue
				}
				storage.EnsureStandardObject(&org, row[4])
				want = append(want, strings.Join(row[3:8], "|"))
			}
			if len(want) != 28 {
				t.Fatal("incomplete native describe cell")
			}
			sort.Strings(want)
			machine := New(nil)
			machine.SetOrg(&org)
			for _, route := range []string{"first", "cached"} {
				t.Run(route, func(t *testing.T) {
					var got []string
					for _, value := range machine.describeChildRelationships("LinkProbe__c") {
						got = append(got, fmt.Sprintf("%s|%s|%s|%t|%t", value.Fields["relationshipName"].Text,
							value.Fields["childSObject"].Fields["object"].Text, value.Fields["field"].Fields["field"].Text,
							value.Fields["cascadeDelete"].Bool, value.Fields["restrictedDelete"].Bool))
					}
					sort.Strings(got)
					if !reflect.DeepEqual(want, got) {
						t.Fatalf("native describe (%d entries)=%v; Glade (%d entries)=%v", len(want), want, len(got), got)
					}
				})
			}
		})
	}
}

func TestCustomPlatformNativeQueriedEmptyChildAccessor(t *testing.T) {
	rows := customPlatformDescribeNativeRows(t)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api+"/getSObjects", func(t *testing.T) {
			org := storage.NewOrgState()
			org.APIVersion = api
			org.Objects["LinkProbe__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "LinkProbe__c"}}
			for _, row := range rows {
				if row[0] == api && row[1] == "anonymous" && row[2] == "describe" {
					storage.EnsureStandardObject(&org, row[4])
				}
			}
			machine := New(nil)
			machine.SetOrg(&org)
			for _, row := range rows {
				if row[0] != api || row[1] != "anonymous" || row[2] != "query" {
					continue
				}
				if row[8] != "reject" || row[9] != "System.NullPointerException" {
					t.Fatal("native empty accessor observation changed")
				}
				t.Run(row[3], func(t *testing.T) {
					parent := Object("LinkProbe__c")
					parent.Fields[row[3]] = List()
					parent.Fields[sobjectQueriedFieldsField] = queriedSObjectFieldsValue(parent.Type, map[string]bool{strings.ToLower(row[3]): true})
					value, handled, err := machine.callSObjectMember(parent, "getSObjects", []Value{String(row[3])})
					if err != nil || !handled || value.Kind != ValueNull {
						t.Fatalf("native queried empty child accessor returns null; Glade kind=%s handled=%t err=%v", value.Kind, handled, err)
					}
				})
			}
		})
	}
}
