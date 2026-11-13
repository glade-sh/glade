package sema

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// These contexts have no captured parser diagnostic. The expected serialized
// result is the output of origin/main for each source and API.
func TestReceiverSplitUncapturedDifferential(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		root := t.TempDir()
		path := filepath.Join(root, "ReceiverProbe.cls")
		writeSemaFile(t, path, "public class ReceiverProbe { public static String value() { return 'ok'; } public String instanceValue() { return 'ok'; } public String instanceValue(Integer i) { return 'ok'; } }")
		index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
		for _, source := range []string{
			"System.assertEquals('ok', String.valueOf(String.valueOf(ReceiverProbe.\nvalue())));",
			"System.Integer.\nvalueOf(1);",
			"String x; x = ReceiverProbe. value();",
			"String x = ReceiverProbe. value();",
			"System.debug(ReceiverProbe. value() + '');",
			"System.debug(ReceiverProbe.\nvalue() + '');",
			"System.debug(ReceiverProbe. value() == 'ok');",
			"System.debug(ReceiverProbe\n. value());",
			"ReceiverProbe obj = new ReceiverProbe();\nobj.\ninstanceValue().length();",
			"ReceiverProbe obj = new ReceiverProbe();\nobj.\ninstanceValue(1);",
		} {
			data, err := json.Marshal(AnalyzeAnonymous(index, source, api).Diagnostics)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != "null" {
				t.Errorf("api %s source %q: got %s, want origin/main null", api, source, data)
			}
		}
	}
}

// No @IsTest route was captured. These named test bodies retain origin/main's
// output until that route is observed natively.
func TestReceiverSplitUncapturedTestRoute(t *testing.T) {
	for _, declaration := range []string{
		"@IsTest private class TestProbe { @IsTest static void run() { ReceiverProbe\n.value(); } }",
	} {
		root := t.TempDir()
		probe := filepath.Join(root, "ReceiverProbe.cls")
		test := filepath.Join(root, "TestProbe.cls")
		writeSemaFile(t, probe, "public class ReceiverProbe { public static String value() { return 'ok'; } }")
		writeSemaFile(t, test, declaration)
		index := typesys.Build(project.Project{Root: root, SourceAPIVersion: "67.0", ApexFiles: []string{probe, test}}, schema.Schema{})
		data, err := json.Marshal(Analyze(index).Diagnostics)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "null" {
			t.Errorf("source %q: got %s, want origin/main null", declaration, data)
		}
	}
}

// Named fluent continuations have no captured after-dot diagnostic.
func TestReceiverSplitUncapturedNamedFluent(t *testing.T) {
	root := t.TempDir()
	probe := filepath.Join(root, "ReceiverProbe.cls")
	caller := filepath.Join(root, "Caller.cls")
	writeSemaFile(t, probe, "public class ReceiverProbe { public static String value() { return 'ok'; } }")
	writeSemaFile(t, caller, "public class Caller { public static void run() { ReceiverProbe.\nvalue().toUpperCase(); } }")
	index := typesys.Build(project.Project{Root: root, SourceAPIVersion: "67.0", ApexFiles: []string{probe, caller}}, schema.Schema{})
	data, err := json.Marshal(Analyze(index).Diagnostics)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "null" {
		t.Errorf("got %s, want origin/main null", data)
	}
}

func TestReceiverSplitUncapturedAPI68(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "ReceiverProbe.cls")
	writeSemaFile(t, path, "public class ReceiverProbe { public static String value() { return 'ok'; } }")
	index := typesys.Build(project.Project{Root: root, SourceAPIVersion: "68.0", ApexFiles: []string{path}}, schema.Schema{})
	for _, source := range []string{"ReceiverProbe\n.value();", "ReceiverProbe.\nvalue();"} {
		data, err := json.Marshal(AnalyzeAnonymous(index, source, "68.0").Diagnostics)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "null" {
			t.Errorf("source %q: got %s, want origin/main null", source, data)
		}
	}
}

// These API, completed-block and return contexts have no matching native row.
func TestReceiverSplitUncapturedContexts(t *testing.T) {
	for _, tc := range []struct{ api, source string }{
		{"64.0", "ReceiverProbe\n.value();"},
		{"67.0", "if (true) { }\nReceiverProbe\n.value();"},
	} {
		root := t.TempDir()
		path := filepath.Join(root, "ReceiverProbe.cls")
		writeSemaFile(t, path, "public class ReceiverProbe { public static String value() { return 'ok'; } }")
		index := typesys.Build(project.Project{Root: root, SourceAPIVersion: tc.api, ApexFiles: []string{path}}, schema.Schema{})
		data, err := json.Marshal(AnalyzeAnonymous(index, tc.source, tc.api).Diagnostics)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "null" {
			t.Errorf("api %s source %q: got %s, want origin/main null", tc.api, tc.source, data)
		}
	}
	root := t.TempDir()
	probe, caller := filepath.Join(root, "ReceiverProbe.cls"), filepath.Join(root, "Caller.cls")
	writeSemaFile(t, probe, "public class ReceiverProbe { public static String value() { return 'ok'; } }")
	writeSemaFile(t, caller, "public class Caller { public static String run() { return ReceiverProbe\n.value(); } public static void other() { ReceiverProbe. value(); } }")
	index := typesys.Build(project.Project{Root: root, SourceAPIVersion: "67.0", ApexFiles: []string{probe, caller}}, schema.Schema{})
	data, err := json.Marshal(Analyze(index).Diagnostics)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "null" {
		t.Errorf("return context: got %s, want origin/main null", data)
	}
}
