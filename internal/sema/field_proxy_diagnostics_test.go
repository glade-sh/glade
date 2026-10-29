package sema

import (
	"encoding/csv"
	"os"
	"strings"
	"testing"
)

func fieldProxyRows(t *testing.T) [][]string {
	t.Helper()
	data, err := os.ReadFile("../../testdata/conformance/field_proxy_diagnostics/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(data)))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 5 {
		t.Fatalf("native rows=%d: %v", len(rows), err)
	}
	return rows[1:]
}

func TestFieldProxyNativeDiagnosticRoutes(t *testing.T) {
	for _, row := range fieldProxyRows(t) {
		for _, expression := range []string{"Schema.Account.fields.getMap()", "Account.fields.getMap()", "Schema.Account.fields.get('Name')", "Account.fields.get('Name')"} {
			for _, route := range []string{row[1], "named", "trigger"} {
				t.Run(strings.Join(row[:3], "/")+"/"+expression+"/"+route, func(t *testing.T) {
					body := "Object value = " + expression + ";"
					source, suffix := "public class ReceiverMatrix { public static void observe() { "+body+" } }", ".cls"
					if route == "isTest" {
						source = "@IsTest private class ReceiverMatrix { @IsTest static void observe() { " + body + " } }"
					}
					if route == "trigger" {
						source = "trigger ReceiverMatrix on Account (before insert) { " + body + " }"
						suffix = ".trigger"
					}
					if route == "anonymous" {
						source = "public class ReceiverMatrix {}"
					}
					index := qualifiedReceiverIndex(t, row[0], "", "present", source, suffix)
					result := Analyze(index)
					if route == "anonymous" {
						result = AnalyzeAnonymous(index, body, row[0])
					}
					want := row[4]
					if strings.Contains(expression, "get(") {
						want = strings.ReplaceAll(want, "getMap()", "get(String)")
					}
					if len(result.Diagnostics) != 1 || result.Diagnostics[0].NativeMessage != want {
						t.Fatalf("native diagnostic %q; got %+v", want, result.Diagnostics)
					}
				})
			}
		}
	}
}
