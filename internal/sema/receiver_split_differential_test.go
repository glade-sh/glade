package sema

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Captured bound-dot and free-dot contexts replace the earlier baseline:
// org.tsv:18-27, 38-41, 52-61, 88-109 and grammar/org.tsv:2-47.
func TestReceiverSplitUncapturedDifferential(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		root := t.TempDir()
		path := filepath.Join(root, "ReceiverProbe.cls")
		writeSemaFile(t, path, "public class ReceiverProbe { public static String value() { return 'ok'; } public static String value(Integer n) { return 'ok'; } public String instanceValue() { return 'ok'; } public String instanceValue(Integer i) { return 'ok'; } }")
		index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
		for _, tc := range []struct{ source, want string }{
			{"System.assertEquals('ok', String.valueOf(String.valueOf(ReceiverProbe.\nvalue())));", "Unexpected token '('."},
			{"System.Integer.\nvalueOf(1);", "Unexpected token 'valueOf'."},
			{"String x; x = ReceiverProbe. value();", "Unexpected token '='."},
			{"String x = ReceiverProbe. value();", "Unexpected token 'x'."},
			{"System.debug(ReceiverProbe. value() + '');", "Unexpected token '('."},
			{"System.debug(ReceiverProbe.\nvalue() + '');", "Unexpected token '('."},
			{"System.debug(ReceiverProbe. value() == 'ok');", "Unexpected token '('."},
			{"System.debug(ReceiverProbe\n. value());", "Variable does not exist: ReceiverProbe"},
			{"System.debug(ReceiverProbe . value());", "Variable does not exist: ReceiverProbe"},
			{"System.debug(ReceiverProbe\n.value() + '');", "Variable does not exist: ReceiverProbe"},
			{"System.debug(ReceiverProbe\n.value(1));", "Variable does not exist: ReceiverProbe"},
			{"System.debug(ReceiverProbe\n.value().toUpperCase());", "Variable does not exist: ReceiverProbe"},
			{"String x = ReceiverProbe .value();", "Variable does not exist: ReceiverProbe"},
			{"ReceiverProbe .value();", "Variable does not exist: ReceiverProbe"},
			{"String x; x = ReceiverProbe\n.value();", "Variable does not exist: ReceiverProbe"},
			{"System.debug(ReceiverProbe\n.value());", "Variable does not exist: ReceiverProbe"},
			{"System.debug(ReceiverProbe.\nvalue());", "Unexpected token '('."},
			{"ReceiverProbe /* note */ .value(1);", "Variable does not exist: ReceiverProbe"},
			{"ReceiverProbe /* note */\n.value();", "Variable does not exist: ReceiverProbe"},
			{"ReceiverProbe./* note */\nvalue();", "Invalid type: ReceiverProbe."},
			{"String x = ReceiverProbe./* note */\nvalue();", "Unexpected token 'x'."},
			{"ReceiverProbe.\nvalue(1);", "Unexpected token 'value'."},
			{"String.\nvalueOf(1);", "Unexpected token 'valueOf'."},
			{"System.\nassert(true);", "Unexpected token 'assert'."},
			{"System.\nnow();", "Invalid type: System."},
			{"System.\ndebug(1);", "Unexpected token 'debug'."},
			{"Integer y = 1; System.\ndebug(y);", "Unexpected token 'debug'."},
			{"System./* note */\ndebug('x');", "Unexpected token 'debug'."},
			{"System.\ndebug(LoggingLevel.INFO, 'x');", "Unexpected token 'debug'."},
			{"String x = ReceiverProbe\n.value(1);", "Variable does not exist: ReceiverProbe"},
			{"String x = ReceiverProbe\n.value().toUpperCase();", "Variable does not exist: ReceiverProbe"},
			{"String x = ReceiverProbe.\nvalue().toUpperCase();", "Unexpected token 'x'."},
			{"String x = ReceiverProbe.\nvalue() + '';", "Unexpected token 'x'."},
			{"String x; x = ReceiverProbe.\nvalue() + '';", "Unexpected token '='."},
			{"String x = 'x' + ReceiverProbe.\nvalue();", "Unexpected token 'x'."},
			{"String x = 'x' + ReceiverProbe\n.value();", "Variable does not exist: ReceiverProbe"},
			{"System.assertEquals('ok', 'x' + ReceiverProbe.\nvalue());", "Unexpected token '('."},
			{"System.assertEquals('ok', 'x' + ReceiverProbe\n.value());", "Variable does not exist: ReceiverProbe"},
			{"ReceiverProbe\n.value().toUpperCase().trim();", "Variable does not exist: ReceiverProbe"},
			{"ReceiverProbe.\nvalue().toUpperCase().trim();", "Unexpected token 'value'."},
			{"System.assertEquals('ok', String.valueOf(ReceiverProbe. value()));", "Unexpected token '('."},
			{"ReceiverProbe obj = new ReceiverProbe();\nobj.\ninstanceValue().length();", "Unexpected token 'instanceValue'."},
			{"ReceiverProbe obj = new ReceiverProbe();\nobj.\ninstanceValue(1);", "Unexpected token 'instanceValue'."},
		} {
			result := AnalyzeAnonymous(index, tc.source, api)
			if len(result.Diagnostics) != 1 || result.Diagnostics[0].NativeMessage != tc.want {
				t.Errorf("api %s source %q: got %v, want %q", api, tc.source, result.Diagnostics, tc.want)
			}
		}
	}
}

// CB covers free-dot expressions in class bodies (grammar/org.tsv:50-51).
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
		got := Analyze(index).Diagnostics
		if len(got) != 1 || got[0].NativeMessage != "Variable does not exist: ReceiverProbe" {
			t.Errorf("source %q: got %v, want captured receiver error", declaration, got)
		}
	}
}

// N1 and D2 capture a bound dot at chain depth and in a class body
// (grammar/org.tsv:24-25, 54-55).
func TestReceiverSplitUncapturedNamedFluent(t *testing.T) {
	root := t.TempDir()
	probe := filepath.Join(root, "ReceiverProbe.cls")
	caller := filepath.Join(root, "Caller.cls")
	writeSemaFile(t, probe, "public class ReceiverProbe { public static String value() { return 'ok'; } }")
	writeSemaFile(t, caller, "public class Caller { public static void run() { ReceiverProbe.\nvalue().toUpperCase(); } }")
	index := typesys.Build(project.Project{Root: root, SourceAPIVersion: "67.0", ApexFiles: []string{probe, caller}}, schema.Schema{})
	got := Analyze(index).Diagnostics
	if len(got) != 1 || got[0].NativeMessage != "Missing ';' at 'value'" {
		t.Errorf("got %v, want captured block message", got)
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

// M6 and CB capture completed-block and return contexts (grammar/org.tsv:20-21,
// 50-51); API 64 lies between the identical native endpoints.
func TestReceiverSplitUncapturedContexts(t *testing.T) {
	for _, tc := range []struct{ api, source string }{
		{"64.0", "ReceiverProbe\n.value();"},
		{"67.0", "if (true) { }\nReceiverProbe\n.value();"},
	} {
		root := t.TempDir()
		path := filepath.Join(root, "ReceiverProbe.cls")
		writeSemaFile(t, path, "public class ReceiverProbe { public static String value() { return 'ok'; } }")
		index := typesys.Build(project.Project{Root: root, SourceAPIVersion: tc.api, ApexFiles: []string{path}}, schema.Schema{})
		got := AnalyzeAnonymous(index, tc.source, tc.api).Diagnostics
		if len(got) != 1 || got[0].NativeMessage != "Variable does not exist: ReceiverProbe" {
			t.Errorf("api %s source %q: got %v, want captured receiver error", tc.api, tc.source, got)
		}
	}
	root := t.TempDir()
	probe, caller := filepath.Join(root, "ReceiverProbe.cls"), filepath.Join(root, "Caller.cls")
	writeSemaFile(t, probe, "public class ReceiverProbe { public static String value() { return 'ok'; } }")
	writeSemaFile(t, caller, "public class Caller { public static String run() { return ReceiverProbe\n.value(); } public static void other() { ReceiverProbe. value(); } }")
	index := typesys.Build(project.Project{Root: root, SourceAPIVersion: "67.0", ApexFiles: []string{probe, caller}}, schema.Schema{})
	got := Analyze(index).Diagnostics
	if len(got) != 2 || got[0].NativeMessage != "Variable does not exist: ReceiverProbe" || got[1].NativeMessage != "Missing ';' at 'value'" {
		t.Errorf("return context: got %v, want captured receiver errors", got)
	}
}
