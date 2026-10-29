package sema

import (
	"os"
	"testing"
)

func TestQualifiedReceiverMissingObjectNamedIsTestTriggerNativeRoutes(t *testing.T) {
	const dir = "testdata/qualified_receiver_widening"
	body, err := os.ReadFile(dir + "/schema_missing_object_fields.apex")
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range readQualifiedReceiverTSV(t, dir+"/org.tsv", 4)[1:] {
		if row[1] != "schema_missing_object_fields" {
			continue
		}
		if row[2] != "false" {
			t.Fatalf("native row is not a rejection: %v", row)
		}
		for _, route := range []string{"named", "isTest", "trigger"} {
			t.Run(row[0]+"/"+route, func(t *testing.T) {
				file := "MissingObjectFields.cls"
				source := "public class MissingObjectFields { public static void observe() {" + string(body) + "} }"
				if route == "isTest" {
					source = "@IsTest private class MissingObjectFields { @IsTest static void observe() {" + string(body) + "} }"
				}
				if route == "trigger" {
					file = "MissingObjectFields.trigger"
					source = "trigger MissingObjectFields on Account (before insert) {" + string(body) + "}"
				}
				result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{file: source}, row[0])
				if !result.HasErrors() {
					t.Fatalf("native rejected (%s); %s route accepted", row[3], route)
				}
			})
		}
	}
}
