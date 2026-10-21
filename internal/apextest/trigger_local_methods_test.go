package apextest

import (
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestTriggerExecutableSourceSeparatesImmediateMethods(t *testing.T) {
	source := "trigger LocalMemberProbe on Account (before insert) {\r\n" +
		"    String label = 'preserved';\r\n" +
		"    private void Decorate(String text) {\r\n" +
		"        String braces = '} { Ω'; // } does not close the helper\r\n" +
		"        if (text != null) { System.debug(braces); }\r\n" +
		"    }\r\n" +
		"    // preserved between methods\r\n" +
		"    Integer amount = 2;\r\n" +
		"    public Integer CountRows() { return Trigger.new.size(); }\r\n" +
		"    System.debug(label);\r\n" +
		"}"
	trigger := triggerSymbolForExtraction(t, source)
	got, names, err := triggerExecutableSource(source, trigger)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(names, []string{"Decorate", "CountRows"}) {
		t.Fatalf("method names=%q", names)
	}
	firstStart := strings.Index(source, "private void")
	firstEnd := strings.Index(source, "    // preserved between methods")
	secondStart := strings.Index(source, "public Integer")
	secondEnd := strings.Index(source, "    System.debug(label)")
	if len(got) != len(source) {
		t.Fatalf("source byte length changed: %d != %d", len(got), len(source))
	}
	for i := range []byte(source) {
		want := source[i]
		if (i >= firstStart && i < firstEnd || i >= secondStart && i < secondEnd) && want != '\r' && want != '\n' {
			want = ' '
		}
		if got[i] != want {
			t.Fatalf("byte %d=%q, want %q", i, got[i], want)
		}
	}
}

func TestTriggerExecutableSourceKeepsStatementsAndNestedMembers(t *testing.T) {
	for _, source := range []string{
		"trigger LocalMemberProbe on Account (before insert) { String text='private void Fake() {}'; System.debug(text); }",
		"trigger LocalMemberProbe on Account (before insert) { /* private void Fake() {} */ if (true) { Integer count=1; } }",
		"trigger LocalMemberProbe on Account (before insert) { class Nested { public void Keep() {} } System.debug('kept'); }",
	} {
		trigger := triggerSymbolForExtraction(t, source)
		got, names, err := triggerExecutableSource(source, trigger)
		if err != nil {
			t.Fatal(err)
		}
		if got != source || names != nil {
			t.Fatalf("non-method trigger source changed: names=%q, source=%q", names, got)
		}
	}
}

func TestTriggerExecutableSourceRejectsMalformedOrReservedSource(t *testing.T) {
	for _, source := range []string{
		"trigger LocalMemberProbe on Account (before insert) { private void Broken( { } }",
		"trigger LocalMemberProbe on Account (before insert) { Integer void=1; private void Helper() {} }",
		"trigger LocalMemberProbe on Account (before insert) { private void Helper() { String currency='USD'; } }",
	} {
		got, names, err := triggerExecutableSource(source, typesys.TriggerSymbol{Name: "LocalMemberProbe", File: "LocalMemberProbe.trigger"})
		if err == nil || got != source || names != nil {
			t.Fatalf("invalid source was masked: error=%v, names=%q", err, names)
		}
	}
}

func TestTriggerExecutableSourceRejectsMismatchedIdentityOrRanges(t *testing.T) {
	source := "trigger LocalMemberProbe on Account (before insert) { private void Helper() {} }"
	original := triggerSymbolForExtraction(t, source)
	for _, name := range []string{"name", "object", "declarationRange", "bodyRange"} {
		t.Run(name, func(t *testing.T) {
			trigger := original
			bodyRange := *original.BodyRange
			trigger.BodyRange = &bodyRange
			switch name {
			case "name":
				trigger.Name = "DifferentProbe"
			case "object":
				trigger.ObjectName = "Contact"
			case "declarationRange":
				trigger.Range.End.Offset--
			case "bodyRange":
				trigger.BodyRange.Start.Offset++
			}
			got, names, err := triggerExecutableSource(source, trigger)
			if err == nil || got != source || names != nil {
				t.Fatalf("mismatched source was masked: error=%v, names=%q", err, names)
			}
		})
	}
}

func triggerSymbolForExtraction(t *testing.T, source string) typesys.TriggerSymbol {
	t.Helper()
	file := apexast.ParseSource("LocalMemberProbe.trigger", source)
	if len(file.Diagnostics) != 0 || len(file.Declarations) != 1 || file.Declarations[0].Kind != apexast.DeclarationTrigger {
		t.Fatalf("synthetic trigger declaration: %#v", file)
	}
	decl := file.Declarations[0]
	return typesys.TriggerSymbol{Name: decl.Name, ObjectName: decl.ObjectName, File: file.Path, Range: decl.Range, BodyRange: decl.BodyRange}
}
