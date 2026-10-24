package vm

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppAnalyticsNativeAnonymousAndIsTestRoutes(t *testing.T) {
	runHostedBoundaryNativeVM(t, "analytics_", 44)
}

func TestCustomNotificationNativeAnonymousAndIsTestRoutes(t *testing.T) {
	runHostedBoundaryNativeVM(t, "notice_", 32)
}

func runHostedBoundaryNativeVM(t *testing.T, prefix string, expected int) {
	t.Helper()
	dir := "../apextest/testdata/conformance/hosted_boundaries"
	raw, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct{ ID, Code string }
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	sources := map[string]string{}
	for _, row := range cases {
		sources[row.ID] = row.Code
	}
	f, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma, reader.LazyQuotes = '\t', true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, row := range rows[1:] {
		if !strings.HasPrefix(row[2], prefix) || (row[1] != "anonymous" && row[1] != "istest") {
			continue
		}
		count++
		t.Run(strings.Join(row[:3], "/"), func(t *testing.T) {
			var want struct{ Result string }
			if err := json.Unmarshal([]byte(row[3]), &want); err != nil || sources[row[2]] == "" {
				t.Fatalf("invalid native row: %v", row)
			}
			if row[1] != "istest" && (row[2] == "notice_configured" || row[2] == "notice_constructor" || row[2] == "notice_page") {
				want.Result = "System.UnsupportedOperationException|Messaging.CustomNotification.send requires hosted notification delivery"
			}
			program, err := CompileAnonymousWithOptions(`String typeId='0ML000000000001'; String observed; try { `+sources[row[2]]+`
observed='returned'; } catch(Exception e) { observed=e.getTypeName()+'|'+e.getMessage(); }
System.debug(observed);`, CompileOptions{APIVersion: row[0]})
			if err != nil {
				t.Fatal(err)
			}
			machine := customNotificationContextVM(row[1] == "istest")
			if err := machine.RegisterClass(Class{Name: "HostedBoundaryProbe.Gesture", EnumValues: []string{"Opened"}, APIVersion: row[0]}); err != nil {
				t.Fatal(err)
			}
			if row[1] == "istest" {
				machine.EnableTestContext()
			}
			result, err := machine.Execute(program)
			if err != nil || len(result.Debug) != 1 || result.Debug[0] != want.Result {
				t.Fatalf("native=%s; Glade=%v; error=%v", want.Result, result.Debug, err)
			}
		})
	}
	if count != expected {
		t.Fatalf("incomplete route matrix: %d, want %d", count, expected)
	}
}
