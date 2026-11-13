package sema

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// receiverSplitNeighbors are multi-line call shapes with no native row. Each
// must produce the diagnostics Glade produces without the receiver-split
// rule, which is origin/main's output.
var receiverSplitNeighbors = []string{
	// standalone and argument calls with arguments on the split call
	"String\n.valueOf('x');",
	"System.assertEquals('x', String\n.valueOf('x'));",
	"System.assertEquals('ok', System.String\n.valueOf('ok'));",
	"System.assertEquals('ok', ReceiverProbe\n.value(), 'msg');",
	// plain newline before the dot in uncaptured contexts
	"System.assertEquals('ok', ReceiverProbe\n.value());",
	"String x = ReceiverProbe\n.value();",
	"ReceiverProbe\n\n.value();",
	"Assert.areEqual('ok', ReceiverProbe\n.value());",
	// repeated after-dot and mixed repeats
	"System.assertEquals(ReceiverProbe\n.value(), ReceiverProbe.\nvalue());",
	"System.assertEquals(ReceiverProbe.\nvalue(), ReceiverProbe\n.value());",
	"System.assertEquals(ReceiverProbe.\nvalue(), ReceiverProbe.\nvalue(), 'msg');",
	"String.valueOf(ReceiverProbe.\nvalue() + ReceiverProbe.\nvalue());",
	// chained calls
	"ReceiverProbe\n.value().toUpperCase();",
	"ReceiverProbe.\nvalue().toUpperCase();",
	"ReceiverProbe.value()\n.toUpperCase();",
	"System.assertEquals('OK', ReceiverProbe\n.value()\n.toUpperCase());",
	"Integer n = ReceiverProbe.\nvalue().length();",
	// comments
	"ReceiverProbe // note\n.value();",
	"ReceiverProbe./* a */value();",
	"System.assertEquals('ok', ReceiverProbe.// comment\nvalue());",
	"System.assertEquals('ok', ReceiverProbe /* a\nb */ .value());",
	"String x = ReceiverProbe./* a */\nvalue();",
	// blank lines
	"ReceiverProbe.\n\nvalue();",
	"System.assertEquals('ok', ReceiverProbe.\n\nvalue());",
	"System.assertEquals('ok', ReceiverProbe\n\n\n.value());",
	// instance receivers
	"ReceiverProbe obj = new ReceiverProbe();\nobj\n.instanceValue();",
	"ReceiverProbe obj = new ReceiverProbe();\nString y = obj\n.instanceValue();",
	"ReceiverProbe obj = new ReceiverProbe();\nobj . instanceValue();",
	"ReceiverProbe obj = new ReceiverProbe();\nSystem.assertEquals('ok', obj./* a\nb */instanceValue());",
	"ReceiverProbe obj = new ReceiverProbe();\nobj.\ninstanceValue().length();",
	// namespace-qualified classes
	"System.assertEquals('ok', nsx.ReceiverProbe\n.value());",
	"System.assertEquals('1', nsx.ReceiverProbe\n.value(1));",
	"nsx.ReceiverProbe\n.value();",
	"nsx.ReceiverProbe.\nvalue();",
	"System.assertEquals('1', System.Integer\n.valueOf(1));",
	// assignments and nesting beyond the captures
	"String x;\nx = ReceiverProbe.\nvalue();",
	"System.assertEquals('ok', String.valueOf(String.valueOf(ReceiverProbe.\nvalue())));",
	"System.assertEquals('ok', ReceiverProbe.\nvalue() + '');",
	"if (true) { ReceiverProbe.\nvalue(); }",
}

func TestReceiverSplitUncapturedDifferential(t *testing.T) {
	for _, namespace := range []string{"", "nsx"} {
		for _, api := range []string{"62.0", "67.0"} {
			root := t.TempDir()
			path := filepath.Join(root, "ReceiverProbe.cls")
			writeSemaFile(t, path, "public class ReceiverProbe { public static String value() { return 'ok'; } public static String value(Integer i) { return 'ok'; } public String instanceValue() { return 'ok'; } }")
			index := typesys.Build(project.Project{Root: root, Namespace: namespace, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
			for _, source := range receiverSplitNeighbors {
				got := receiverSplitDiagnosticsJSON(t, index, source, api)
				saved := splitReceiverShapes
				splitReceiverShapes = nil
				want := receiverSplitDiagnosticsJSON(t, index, source, api)
				splitReceiverShapes = saved
				if got != want {
					t.Errorf("ns %q api %s source %q:\n got %s\nwant %s", namespace, api, source, got, want)
				}
			}
		}
	}
}

func receiverSplitDiagnosticsJSON(t *testing.T, index typesys.Index, source, api string) string {
	t.Helper()
	data, err := json.Marshal(AnalyzeAnonymous(index, source, api).Diagnostics)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
