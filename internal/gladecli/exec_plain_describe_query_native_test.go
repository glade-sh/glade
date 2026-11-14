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
					checks.WriteString("try { Schema.describeSObjects(new List<String>{'" + cols[3] + "'}); } catch (Exception e) {}\n")
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
					body.WriteString("try { Schema.describeSObjects(new List<String>{'" + cols[3] + "'}); } catch (Exception e) {}\n")
				case "query":
					body.WriteString("try { Database.query('SELECT Id FROM " + cols[3] + " LIMIT 1'); System.assert(false, 'query succeeded'); } catch (QueryException e) { System.assert(e.getMessage().contains('is not supported')); }\n")
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
		})
	}
}
