package apextest

import (
	"errors"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// Synthetic route twins for the system LastModifiedDate audit fallback,
// through real anonymous Apex and @IsTest execution of record-triggered Flows
// at API 62.0 and 67.0.
//
// par: the parent Update Records assignment from $Record.LastModifiedDate is
// the native-backed positive control (internal/apextest/testdata/conformance/
// flow_decision_shapes/org.tsv, API 62.0/67.0 anonymous and istest,
// PARENT:Open dt=set).
// sub and hit: subflow input binding and a Get Records filter read the same
// reference. No native row observes either, so they keep the Glade base
// (a recorded revision) resolution of a system-only timestamp as null: the leaf writes
// null (sub=null, ran=ran) and the null-equality filter still matches the
// unset Aud_Mark__c (hit=found). These rows guard base behavior; they do not
// claim native parity.
const flowAuditRouteWant = "AUDROWS|par=set|sub=null|ran=ran|hit=found"

func flowAuditRouteFiles(api string) (map[string]string, map[string]string) {
	field := func(name, kind string) string {
		extra := ""
		if kind == "Text" {
			extra = "<length>20</length>"
		}
		return `<?xml version="1.0" encoding="UTF-8"?>
<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>` + name + `</fullName><label>` + name + `</label>` + extra + `<type>` + kind + `</type></CustomField>`
	}
	flow := func(body string) string {
		return `<?xml version="1.0" encoding="UTF-8"?>
<Flow xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>` + api + `</apiVersion>` + body + `<status>Active</status></Flow>`
	}
	start := func(target string) string {
		return `<start><locationX>0</locationX><locationY>0</locationY><connector><targetReference>` + target + `</targetReference></connector><filterLogic>and</filterLogic><filters><field>LastName</field><operator>EqualTo</operator><value><stringValue>Aud</stringValue></value></filters><object>Contact</object><recordTriggerType>Create</recordTriggerType><triggerType>RecordAfterSave</triggerType></start>`
	}
	updateAccount := func(name, assignments string) string {
		return `<recordUpdates><name>` + name + `</name><label>` + name + `</label><locationX>0</locationX><locationY>0</locationY><filterLogic>and</filterLogic><filters><field>Id</field><operator>EqualTo</operator><value><elementReference>$Record.Aud_Acct__c</elementReference></value></filters>` + assignments + `<object>Account</object></recordUpdates>`
	}
	prefix := "force-app/main/default/"
	files := map[string]string{
		prefix + "objects/Account/fields/Aud_Par__c.field-meta.xml":    field("Aud_Par__c", "DateTime"),
		prefix + "objects/Account/fields/Aud_Sub__c.field-meta.xml":    field("Aud_Sub__c", "DateTime"),
		prefix + "objects/Account/fields/Aud_Mark__c.field-meta.xml":   field("Aud_Mark__c", "DateTime"),
		prefix + "objects/Account/fields/Aud_SubRan__c.field-meta.xml": field("Aud_SubRan__c", "Text"),
		prefix + "objects/Account/fields/Aud_Hit__c.field-meta.xml":    field("Aud_Hit__c", "Text"),
		prefix + "objects/Contact/fields/Aud_Acct__c.field-meta.xml": `<?xml version="1.0" encoding="UTF-8"?>
<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Aud_Acct__c</fullName><label>Aud_Acct__c</label><referenceTo>Account</referenceTo><relationshipLabel>AudKids</relationshipLabel><relationshipName>AudKids</relationshipName><deleteConstraint>SetNull</deleteConstraint><type>Lookup</type></CustomField>`,
		// Native shape of the parent-update capture (Linden_ParentUpd).
		prefix + "flows/Aud_Parent.flow-meta.xml": flow(`<label>Aud_Parent</label><processType>AutoLaunchedFlow</processType>` +
			`<recordUpdates><name>Upd_Parent</name><label>Upd Parent</label><locationX>0</locationX><locationY>0</locationY><inputAssignments><field>Aud_Par__c</field><value><elementReference>$Record.LastModifiedDate</elementReference></value></inputAssignments><inputReference>$Record.Aud_Acct__r</inputReference></recordUpdates>` +
			start("Upd_Parent")),
		prefix + "flows/Aud_Caller.flow-meta.xml": flow(`<label>Aud_Caller</label><processType>AutoLaunchedFlow</processType>` +
			start("CallLeaf") +
			`<subflows><name>CallLeaf</name><label>Call leaf</label><locationX>0</locationX><locationY>0</locationY><flowName>Aud_Leaf</flowName><inputAssignments><name>Stamp</name><value><elementReference>$Record.LastModifiedDate</elementReference></value></inputAssignments><inputAssignments><name>Acct</name><value><elementReference>$Record.Aud_Acct__c</elementReference></value></inputAssignments></subflows>`),
		prefix + "flows/Aud_Leaf.flow-meta.xml": flow(`<label>Aud_Leaf</label><processType>AutoLaunchedFlow</processType>` +
			`<recordUpdates><name>Write</name><label>Write</label><locationX>0</locationX><locationY>0</locationY><filterLogic>and</filterLogic><filters><field>Id</field><operator>EqualTo</operator><value><elementReference>Acct</elementReference></value></filters><inputAssignments><field>Aud_Sub__c</field><value><elementReference>Stamp</elementReference></value></inputAssignments><inputAssignments><field>Aud_SubRan__c</field><value><stringValue>ran</stringValue></value></inputAssignments><object>Account</object></recordUpdates>` +
			`<start><locationX>0</locationX><locationY>0</locationY><connector><targetReference>Write</targetReference></connector></start>` +
			`<variables><name>Stamp</name><dataType>DateTime</dataType><isCollection>false</isCollection><isInput>true</isInput><isOutput>false</isOutput></variables><variables><name>Acct</name><dataType>String</dataType><isCollection>false</isCollection><isInput>true</isInput><isOutput>false</isOutput></variables>`),
		prefix + "flows/Aud_Lookup.flow-meta.xml": flow(`<label>Aud_Lookup</label><processType>AutoLaunchedFlow</processType>` +
			`<decisions><name>Check</name><label>Check</label><locationX>0</locationX><locationY>0</locationY><defaultConnector><targetReference>MarkMissing</targetReference></defaultConnector><defaultConnectorLabel>Missing</defaultConnectorLabel><rules><name>Found</name><conditionLogic>and</conditionLogic><conditions><leftValueReference>FindMark</leftValueReference><operator>IsNull</operator><rightValue><booleanValue>false</booleanValue></rightValue></conditions><connector><targetReference>MarkFound</targetReference></connector><label>Found</label></rules></decisions>` +
			`<recordLookups><name>FindMark</name><label>FindMark</label><locationX>0</locationX><locationY>0</locationY><assignNullValuesIfNoRecordsFound>true</assignNullValuesIfNoRecordsFound><connector><targetReference>Check</targetReference></connector><filterLogic>and</filterLogic><filters><field>Id</field><operator>EqualTo</operator><value><elementReference>$Record.Aud_Acct__c</elementReference></value></filters><filters><field>Aud_Mark__c</field><operator>EqualTo</operator><value><elementReference>$Record.LastModifiedDate</elementReference></value></filters><getFirstRecordOnly>true</getFirstRecordOnly><object>Account</object><storeOutputAutomatically>true</storeOutputAutomatically></recordLookups>` +
			updateAccount("MarkFound", `<inputAssignments><field>Aud_Hit__c</field><value><stringValue>found</stringValue></value></inputAssignments>`) +
			updateAccount("MarkMissing", `<inputAssignments><field>Aud_Hit__c</field><value><stringValue>missing</stringValue></value></inputAssignments>`) +
			start("FindMark")),
	}
	classes := map[string]string{
		"AudProbe": `public class AudProbe {
    public static void run() {
        Account a = new Account(Name = 'Aud');
        insert a;
        insert new Contact(LastName = 'Aud', Aud_Acct__c = a.Id);
        Account r = [SELECT Aud_Par__c, Aud_Sub__c, Aud_SubRan__c, Aud_Hit__c FROM Account WHERE Id = :a.Id];
        System.assert(false, 'AUDROWS|par=' + (r.Aud_Par__c == null ? 'null' : 'set') + '|sub=' + (r.Aud_Sub__c == null ? 'null' : 'set') + '|ran=' + r.Aud_SubRan__c + '|hit=' + r.Aud_Hit__c);
    }
}`,
		"AudProbeTest": `@IsTest
private class AudProbeTest {
    @IsTest
    static void observe() {
        AudProbe.run();
    }
}`,
	}
	return classes, files
}

func TestFlowAuditTimestampAnonymousAndIsTestRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			classes, files := flowAuditRouteFiles(api)
			index := flowRefsIndex(t, api, classes, files)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("synthetic declarations rejected: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			for _, route := range []string{"anonymous", "istest"} {
				t.Run(route, func(t *testing.T) {
					var message string
					if route == "anonymous" {
						org := orgFromIndex(index)
						org.APIVersion = api
						runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
						program, err := vm.CompileAnonymousWithOptions("AudProbe.run();", vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						_, err = runner.newMachine().Execute(program)
						var runtimeErr *vm.RuntimeError
						if !errors.As(err, &runtimeErr) || flowRefsExceptionType(runtimeErr.Type) != "System.AssertException" {
							t.Fatalf("missing observation assertion: %v", err)
						}
						message = runtimeErr.ExceptionMessage()
					} else {
						result := Run(index, Options{SelectedClasses: []string{"AudProbeTest"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
						if result.Summary().Total != 1 || len(result.Suites) != 1 || len(result.Suites[0].Cases) != 1 {
							t.Fatalf("unexpected test selection: %#v", result.Summary())
						}
						observed := result.Suites[0].Cases[0]
						if observed.Problem == nil || flowRefsExceptionType(observed.Problem.Type) != "System.AssertException" {
							t.Fatalf("missing observation assertion: %#v", observed.Problem)
						}
						message = observed.Problem.Message
					}
					_, rows, found := strings.Cut(message, "AUDROWS|")
					if !found {
						t.Fatalf("missing observation envelope: %s", message)
					}
					if got := "AUDROWS|" + strings.TrimSpace(rows); got != flowAuditRouteWant {
						t.Fatalf("Glade %s; want %s", got, flowAuditRouteWant)
					}
				})
			}
		})
	}
}
