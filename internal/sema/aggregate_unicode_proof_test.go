package sema

import (
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
	"path/filepath"
	"strings"
	"testing"
)

func TestSF181AggregateOrderAPI62(t *testing.T) {
	source := `@IsTest private class GladeAggregateOrder62Proof {
 @IsTest static void selectedSumOrderingKeepsAggregateAlias() {
  insert new List<Account>{
   new Account(Name='owned-order-a',AnnualRevenue=10),
   new Account(Name='owned-order-a',AnnualRevenue=20),
   new Account(Name='owned-order-b',AnnualRevenue=5),
   new Account(Name='owned-order-zero',AnnualRevenue=0)
  };
  Set<String> names=new Set<String>{'owned-order-a','owned-order-b','owned-order-zero'};
  List<AggregateResult> observed=new List<AggregateResult>();
  for (List<AggregateResult> listAG : [select Name cId, SUM(AnnualRevenue) sumHours
   from Account
   where Name IN :names
   group by Name
   having SUM(AnnualRevenue) > 0
   order by SUM(AnnualRevenue) desc ]) {
   observed.addAll(listAG);
  }
  System.assertEquals(2,observed.size());
  System.assertEquals('owned-order-a',observed[0].get('cId'));
  System.assertEquals(Decimal.valueOf('30'),observed[0].get('sumHours'));
  System.assertEquals('owned-order-b',observed[1].get('cId'));
  System.assertEquals(Decimal.valueOf('5'),observed[1].get('sumHours'));
 }
}
`
	root := t.TempDir()
	file := filepath.Join(root, "GladeAggregateOrder62Proof.cls")
	writeSemaFile(t, file, source)
	writeSemaFile(t, file+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{file}}, schema.Schema{Objects: []schema.Object{{Name: "Account", Fields: []schema.Field{{Name: "Name", Type: "Text"}, {Name: "AnnualRevenue", Type: "Currency"}}}}})
	index.Project = typesys.ProjectInfo{SourceAPIVersion: "40.0"}
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("aggregate rejected: %#v", result.Diagnostics)
	}
}

func TestSF181UnicodeThisAPI52(t *testing.T) {
	result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"GladeUnicodeThis52Proof.cls": `@IsTest private class GladeUnicodeThis52Proof {
 private static testMethod void unicodePrecursorKeepsThisInsideString() {
  Test.startTest();
  String mainString='ThIs iS hArD tO rEaD';
  String unicodePrecursor='KIYMETLİ';
  String result=mainString.toLowerCase();
  System.AssertEquals('this is hard to read',result);
  System.AssertEquals('KIYMETLİ',unicodePrecursor);
  System.AssertEquals(8,unicodePrecursor.length());
  Test.stopTest();
 }
}
`}, "52.0")
	if result.HasErrors() {
		t.Fatalf("unicode literal rejected: %#v", result.Diagnostics)
	}
}

func TestAggregateOrderGeneratedAliasDoesNotHideUnknownFields(t *testing.T) {
	result := analyzeQueryProbe(t, `public class Probe { void run() {
  List<AggregateResult> good=[SELECT COUNT(Id) total FROM Account ORDER BY total];
  List<AggregateResult> missing=[SELECT COUNT(Id) total FROM Account ORDER BY expr99];
  List<AggregateResult> literal=[SELECT COUNT(Id) total FROM Account ORDER BY expr0];
  List<AggregateResult> field=[SELECT COUNT(Missing__c) total FROM Account ORDER BY COUNT(Missing__c)];
 } }`, queryDiagnosticSchema())
	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "Account.total")
	foundAlias, foundField, foundLiteral := false, false, false
	for _, d := range result.Diagnostics {
		if d.Code == "GLADESEMA_QUERY_FIELD" {
			if strings.Contains(d.Message, "expr0") {
				foundLiteral = true
			}
			if strings.Contains(d.Message, "expr99") {
				foundAlias = true
			}
			if strings.Contains(d.Message, "Missing__c") {
				foundField = true
			}
		}
	}
	if !foundAlias || !foundField || !foundLiteral {
		t.Fatalf("missing expected field errors: %#v", result.Diagnostics)
	}
}

func TestUnicodeStaticThisKeepsOriginalSourceOffsets(t *testing.T) {
	body := "String text='İ'; /* THIS */ System.debug('this'); System.debug(ThIs);"
	typ := typesys.TypeSymbol{Name: "Probe"}
	member := typesys.MemberSymbol{Name: "run", Modifiers: []string{"static"}}
	diagnostics := staticThisDiagnostics(typ, member, body, 0, body)
	if len(diagnostics) != 1 {
		t.Fatalf("diagnostics: %#v", diagnostics)
	}
	want := strings.LastIndex(body, "ThIs")
	if diagnostics[0].Range.Start.Offset != want {
		t.Fatalf("offset=%d want%d", diagnostics[0].Range.Start.Offset, want)
	}
}
