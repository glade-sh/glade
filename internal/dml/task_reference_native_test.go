package dml

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestTaskReferenceNativeDirectEngine(t *testing.T) {
	file, err := os.Open("../apextest/testdata/conformance/task_reference/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma, reader.LazyQuotes = '\t', true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 5 {
		t.Fatalf("native rows: %v; count=%d", err, len(rows))
	}
	for _, row := range rows[1:] {
		if row[1] != "anonymous" {
			continue
		}
		t.Run("api"+row[0], func(t *testing.T) {
			runTaskReferenceNativeDirectEngine(t, row[0], row[2])
		})
	}
}

func runTaskReferenceNativeDirectEngine(t *testing.T, api, observation string) {
	t.Helper()
	var native map[string]struct {
		Status string   `json:"status"`
		Fields []string `json:"fields"`
	}
	if err := json.Unmarshal([]byte(observation), &native); err != nil {
		t.Fatal(err)
	}
	for _, operation := range []string{"insert", "update", "upsertCreate", "upsertUpdate"} {
		for _, kind := range []string{"valid", "missing", "deleted", "wrongType"} {
			for _, loaded := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/loaded=%t", operation, kind, loaded), func(t *testing.T) {
					org := storage.NewOrgState()
					org.APIVersion = api
					for _, name := range []string{"Account", "Contact", "Task"} {
						storage.EnsureStandardObject(&org, name)
					}
					engine := NewEngine(&org)
					parents := engine.Insert([]storage.Record{
						{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("live")}},
						{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("deleted")}},
						{Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("wrong type")}},
					})
					for _, result := range parents {
						if !result.Success {
							t.Fatal(result)
						}
					}
					if result := engine.Delete([]storage.Record{{Object: "Account", ID: parents[1].ID}})[0]; !result.Success {
						t.Fatal(result)
					}
					ids := map[string]storage.ID{"valid": parents[0].ID, "missing": "001000000000123", "deleted": parents[1].ID, "wrongType": parents[2].ID}
					input := storage.Record{Object: "Task", Fields: map[string]storage.Value{"Subject": storage.StringValue("reference")}}
					if operation == "update" || operation == "upsertUpdate" {
						result := engine.Insert([]storage.Record{input})[0]
						if !result.Success {
							t.Fatal(result)
						}
						input.ID = result.ID
					}
					input.Fields["WhatId"] = storage.IDValue(ids[kind])
					if loaded {
						input.LoadedReferences = map[string]storage.ID{"WhatId": ids[kind]}
					}
					var result Result
					if operation == "insert" {
						result = engine.Insert([]storage.Record{input})[0]
					} else if operation == "update" {
						result = engine.Update([]storage.Record{input})[0]
					} else {
						result = engine.Upsert([]storage.Record{input})[0]
					}
					want, ok := native[operation+"/"+kind+"/databaseFalse"]
					if !ok {
						t.Fatal("missing native cell")
					}
					status := result.StatusCode
					if result.Success {
						status = "SUCCESS"
					}
					if status != want.Status || !slices.Equal(result.Fields, want.Fields) {
						t.Fatalf("native status=%s fields=%v; direct Engine status=%s fields=%v", want.Status, want.Fields, status, result.Fields)
					}
				})
			}
		}
	}
}
