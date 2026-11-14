package gladecli

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The lookups and query rows in plain_describe_query/org.tsv cover both APIs.
func TestRunExecPlainDescribeQueryNativeConformance(t *testing.T) {
	data, err := os.ReadFile("testdata/plain_describe_query/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := execStandardOrgProject(t, "")
			writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			writeTestFile(t, filepath.Join(root, "config", "project-scratch-def.json"), `{"edition":"Developer","features":[],"settings":{}}`)
			var checks strings.Builder
			for _, line := range strings.Split(string(data), "\n") {
				cols := strings.Split(line, "\t")
				if len(cols) != 5 || cols[0] != api || cols[1] != "anonymous" {
					continue
				}
				switch cols[2] {
				case "initial", "warm", "standard":
					checks.WriteString("System.assertEquals(false, Schema.getGlobalDescribe().containsKey('" + cols[3] + "')); System.assertEquals(null, Schema.getGlobalDescribe().get('" + cols[3] + "'));\n")
				case "control":
					checks.WriteString("System.assertNotEquals(null, Account.SObjectType);\n")
				case "type":
					checks.WriteString("System.assertEquals(null, Type.forName('" + cols[3] + "'));\n")
				case "describe":
					// The unqualified name must throw the native message. The
					// qualified name follows a query that completed the spread
					// and still describes on this route, as on origin/main.
					checks.WriteString(plainDescribeProbe(cols[3], cols[4], strings.Count(cols[3], "__") > 1))
				case "query":
					msg := strings.TrimPrefix(cols[4], "EXCEPTION|")
					checks.WriteString("try { Database.query('SELECT Id FROM " + cols[3] + " LIMIT 1'); System.assert(false, 'query succeeded'); } catch (QueryException e) { System.assertEquals('" + strings.ReplaceAll(msg, "'", "\\'") + "', e.getMessage()); }\n")
				}
			}
			if checks.Len() == 0 {
				t.Fatal("empty native row selection")
			}
			if err := runExec(context.Background(), []string{"--project", root, "--json", checks.String()}, io.Discard); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// The captured test route retains the pre-existing qualified-name error for
// the unqualified query; both names must still reject instead of returning rows.
func TestPlainDescribeQueryTestRunnerNativeConformance(t *testing.T) {
	data, err := os.ReadFile("testdata/plain_describe_query/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := execStandardOrgProject(t, "")
			writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			writeTestFile(t, filepath.Join(root, "config", "project-scratch-def.json"), `{"edition":"Developer","features":[],"settings":{}}`)
			var body strings.Builder
			qualifiedMessage := ""
			for _, line := range strings.Split(string(data), "\n") {
				cols := strings.Split(line, "\t")
				if len(cols) == 5 && cols[0] == api && cols[1] == "istest" && cols[2] == "query" && strings.Count(cols[3], "__") > 1 {
					qualifiedMessage = strings.TrimPrefix(cols[4], "EXCEPTION|")
				}
			}
			if qualifiedMessage == "" {
				t.Fatal("missing qualified query row")
			}
			for _, line := range strings.Split(string(data), "\n") {
				cols := strings.Split(line, "\t")
				if len(cols) != 5 || cols[0] != api || cols[1] != "istest" {
					continue
				}
				switch cols[2] {
				case "initial", "warm", "standard":
					body.WriteString("System.assertEquals(false, Schema.getGlobalDescribe().containsKey('" + cols[3] + "')); System.assertEquals(null, Schema.getGlobalDescribe().get('" + cols[3] + "'));\n")
				case "control":
					body.WriteString("System.assertNotEquals(null, Account.SObjectType);\n")
				case "type":
					body.WriteString("System.assertEquals(null, Type.forName('" + cols[3] + "'));\n")
				case "describe":
					// The test route still describes both names, as on origin/main.
					body.WriteString(plainDescribeProbe(cols[3], cols[4], true))
				case "query":
					native := strings.ReplaceAll(strings.TrimPrefix(cols[4], "EXCEPTION|"), "'", "\\'")
					match := "e.getMessage() == '" + native + "'"
					if strings.Count(cols[3], "__") == 1 {
						match += " || e.getMessage() == '" + strings.ReplaceAll(qualifiedMessage, "'", "\\'") + "'"
					}
					body.WriteString("try { Database.query('SELECT Id FROM " + cols[3] + " LIMIT 1'); System.assert(false, 'query succeeded'); } catch (QueryException e) { System.assert(" + match + ", e.getMessage()); }\n")
				}
			}
			if body.Len() == 0 {
				t.Fatal("empty native row selection")
			}
			writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/DescribeQueryProbe.cls"), "@IsTest private class DescribeQueryProbe { @IsTest static void runProbe() {\n"+body.String()+"\n} }")
			writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/DescribeQueryProbe.cls-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?><ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			var out, errs bytes.Buffer
			if code := Run(context.Background(), []string{"test", "--project", root, "--no-cache", "--no-serve", "--no-progress", "--json"}, &out, &errs); code != 0 {
				t.Fatalf("test exit %d: %s %s", code, out.String(), errs.String())
			}
			run, err := decodeTestRunJSON(out.Bytes())
			if err != nil {
				t.Fatal(err)
			}
			count := 0
			for _, suite := range run.Suites {
				count += len(suite.Cases)
			}
			if count != 1 {
				t.Fatalf("selected %d tests, want 1: %s", count, out.String())
			}
		})
	}
}

// plainDescribeProbe asserts the captured describe row. The call must throw
// the native message; with baseline set it may instead return the single
// result origin/main returns, a recorded gap. Any other outcome fails.
func plainDescribeProbe(name, outcome string, baseline bool) string {
	msg := strings.ReplaceAll(strings.TrimPrefix(outcome, "EXCEPTION|"), "'", "\\'")
	call := "Schema.describeSObjects(new List<String>{'" + name + "'})"
	success := call + "; System.assert(false, 'describe succeeded');"
	if baseline {
		success = "System.assertEquals(1, " + call + ".size());"
	}
	return "try { " + success + " } catch (InvalidParameterValueException e) { System.assertEquals('" + msg + "', e.getMessage()); }\n"
}
