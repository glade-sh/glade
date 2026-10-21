package apextest

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var invocableFlowIDRE = regexp.MustCompile(`\b[0-9a-zA-Z]{15,18}\b`)

func runInvocableFlowProbe(t *testing.T, class, name string) string {
	t.Helper()
	root := t.TempDir()
	const base = "force-app/main/default/"
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"62.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, base, "objects/Case/fields/Reg__c.field-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<CustomField xmlns="http://soap.sforce.com/2006/04/metadata">
    <fullName>Reg__c</fullName><label>Reg</label><type>MultiselectPicklist</type><visibleLines>4</visibleLines><valueSet><restricted>true</restricted><valueSetDefinition><sorted>false</sorted><value><fullName>Uno</fullName><default>false</default><label>Uno</label></value><value><fullName>Dos</fullName><default>false</default><label>Dos</label></value><value><fullName>Tres</fullName><default>false</default><label>Tres</label></value></valueSetDefinition></valueSet>
</CustomField>
`)
	writeFile(t, filepath.Join(root, base, "objects/Case/fields/Sub__c.field-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<CustomField xmlns="http://soap.sforce.com/2006/04/metadata">
    <fullName>Sub__c</fullName><label>Sub</label><type>Picklist</type><valueSet><restricted>true</restricted><valueSetDefinition><sorted>false</sorted><value><fullName>Uno</fullName><default>false</default><label>Uno</label></value><value><fullName>Dos</fullName><default>false</default><label>Dos</label></value><value><fullName>Tres</fullName><default>false</default><label>Tres</label></value></valueSetDefinition></valueSet>
</CustomField>
`)
	writeFile(t, filepath.Join(root, base, "objects/Case/recordTypes/Sol.recordType-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<RecordType xmlns="http://soap.sforce.com/2006/04/metadata">
    <fullName>Sol</fullName><active>true</active><businessProcess>ProbeBp</businessProcess><label>Solicitud de vía</label>
    <picklistValues><picklist>Status</picklist><values><fullName>New</fullName><default>true</default></values><values><fullName>Closed</fullName><default>false</default></values></picklistValues><picklistValues><picklist>Reg__c</picklist><values><fullName>Uno</fullName><default>false</default></values><values><fullName>Dos</fullName><default>false</default></values><values><fullName>Tres</fullName><default>false</default></values></picklistValues>
    <picklistValues><picklist>Sub__c</picklist><values><fullName>Uno</fullName><default>false</default></values><values><fullName>Dos</fullName><default>false</default></values><values><fullName>Tres</fullName><default>false</default></values></picklistValues>
</RecordType>
`)
	writeFile(t, filepath.Join(root, base, "flows/IndFlow.flow-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
    <apiVersion>62.0</apiVersion>
    <assignments>
        <name>A_adir_subestado_a_registro</name>
        <label>Añadir subestado a registro</label>
        <locationX>182</locationX>
        <locationY>350</locationY>
        <assignmentItems>
            <assignToReference>record.Reg__c</assignToReference>
            <operator>AddItem</operator>
            <value>
                <elementReference>record.Sub__c</elementReference>
            </value>
        </assignmentItems>
    </assignments>
    <decisions>
        <name>Revisar_RecordType</name>
        <label>Revisar RecordType</label>
        <locationX>248</locationX>
        <locationY>134</locationY>
        <defaultConnectorLabel>Default Outcome</defaultConnectorLabel>
        <rules>
            <name>RecordType_Correcto</name>
            <conditionLogic>and</conditionLogic>
            <conditions>
                <leftValueReference>record.RecordType.DeveloperName</leftValueReference>
                <operator>EqualTo</operator>
                <rightValue>
                    <stringValue>Sol</stringValue>
                </rightValue>
            </conditions>
            <connector>
                <targetReference>Ya_tiene_ese_subestado_registrado</targetReference>
            </connector>
            <label>RecordType Correcto</label>
        </rules>
    </decisions>
    <decisions>
        <name>Ya_tiene_ese_subestado_registrado</name>
        <label>¿Ya tiene ese subestado registrado?</label>
        <locationX>50</locationX>
        <locationY>242</locationY>
        <defaultConnector>
            <targetReference>A_adir_subestado_a_registro</targetReference>
        </defaultConnector>
        <defaultConnectorLabel>Default Outcome</defaultConnectorLabel>
        <rules>
            <name>SI</name>
            <conditionLogic>and</conditionLogic>
            <conditions>
                <leftValueReference>record.Reg__c</leftValueReference>
                <operator>Contains</operator>
                <rightValue>
                    <elementReference>record.Sub__c</elementReference>
                </rightValue>
            </conditions>
            <label>SI</label>
        </rules>
    </decisions>
    <description>para corregir el error de usarse after y controlar el recordtype</description>
    <environments>Default</environments>
    <interviewLabel>IndFlow {!$Flow.CurrentDateTime}</interviewLabel>
    <label>IndFlow</label>
    <processMetadataValues>
        <name>BuilderType</name>
        <value>
            <stringValue>LightningFlowBuilder</stringValue>
        </value>
    </processMetadataValues>
    <processMetadataValues>
        <name>CanvasMode</name>
        <value>
            <stringValue>AUTO_LAYOUT_CANVAS</stringValue>
        </value>
    </processMetadataValues>
    <processMetadataValues>
        <name>OriginBuilderType</name>
        <value>
            <stringValue>LightningFlowBuilder</stringValue>
        </value>
    </processMetadataValues>
    <processType>AutoLaunchedFlow</processType>
    <start>
        <locationX>122</locationX>
        <locationY>0</locationY>
        <connector>
            <targetReference>Revisar_RecordType</targetReference>
        </connector>
    </start>
    <status>Active</status>
    <variables>
        <name>caso_aux</name>
        <dataType>SObject</dataType>
        <isCollection>false</isCollection>
        <isInput>false</isInput>
        <isOutput>false</isOutput>
        <objectType>Case</objectType>
    </variables>
    <variables>
        <description>the new version of the record in the DML operation</description>
        <name>record</name>
        <dataType>SObject</dataType>
        <isCollection>false</isCollection>
        <isInput>true</isInput>
        <isOutput>true</isOutput>
        <objectType>Case</objectType>
    </variables>
    <variables>
        <name>recordPrior</name>
        <dataType>SObject</dataType>
        <isCollection>false</isCollection>
        <isInput>true</isInput>
        <isOutput>false</isOutput>
        <objectType>Case</objectType>
    </variables>
</Flow>
`)
	writeFile(t, filepath.Join(root, base, "flows/IndFlow2.flow-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
    <apiVersion>62.0</apiVersion>
    <assignments>
        <name>A_adir_subestado_a_registro</name>
        <label>Añadir subestado a registro</label>
        <locationX>182</locationX>
        <locationY>350</locationY>
        <assignmentItems>
            <assignToReference>record.Subject</assignToReference><operator>Assign</operator><value><stringValue>DEFAULT</stringValue></value>
        </assignmentItems>
    </assignments>
    <decisions>
        <name>Revisar_RecordType</name>
        <label>Revisar RecordType</label>
        <locationX>248</locationX>
        <locationY>134</locationY>
        <defaultConnectorLabel>Default Outcome</defaultConnectorLabel>
        <rules>
            <name>RecordType_Correcto</name>
            <conditionLogic>and</conditionLogic>
            <conditions>
                <leftValueReference>record.RecordType.DeveloperName</leftValueReference>
                <operator>EqualTo</operator>
                <rightValue>
                    <stringValue>Sol</stringValue>
                </rightValue>
            </conditions>
            <connector>
                <targetReference>Ya_tiene_ese_subestado_registrado</targetReference>
            </connector>
            <label>RecordType Correcto</label>
        </rules>
    </decisions>
    <decisions>
        <name>Ya_tiene_ese_subestado_registrado</name>
        <label>¿Ya tiene ese subestado registrado?</label>
        <locationX>50</locationX>
        <locationY>242</locationY>
        <defaultConnector>
            <targetReference>A_adir_subestado_a_registro</targetReference>
        </defaultConnector>
        <defaultConnectorLabel>Default Outcome</defaultConnectorLabel>
        <rules>
            <name>SI</name>
            <conditionLogic>and</conditionLogic>
            <conditions>
                <leftValueReference>record.Reg__c</leftValueReference>
                <operator>Contains</operator>
                <rightValue>
                    <elementReference>record.Sub__c</elementReference>
                </rightValue>
            </conditions>
            <label>SI</label>
        </rules>
    </decisions>
    <description>para corregir el error de usarse after y controlar el recordtype</description>
    <environments>Default</environments>
    <interviewLabel>IndFlow2 {!$Flow.CurrentDateTime}</interviewLabel>
    <label>IndFlow2</label>
    <processMetadataValues>
        <name>BuilderType</name>
        <value>
            <stringValue>LightningFlowBuilder</stringValue>
        </value>
    </processMetadataValues>
    <processMetadataValues>
        <name>CanvasMode</name>
        <value>
            <stringValue>AUTO_LAYOUT_CANVAS</stringValue>
        </value>
    </processMetadataValues>
    <processMetadataValues>
        <name>OriginBuilderType</name>
        <value>
            <stringValue>LightningFlowBuilder</stringValue>
        </value>
    </processMetadataValues>
    <processType>AutoLaunchedFlow</processType>
    <start>
        <locationX>122</locationX>
        <locationY>0</locationY>
        <connector>
            <targetReference>Revisar_RecordType</targetReference>
        </connector>
    </start>
    <status>Active</status>
    <variables>
        <name>caso_aux</name>
        <dataType>SObject</dataType>
        <isCollection>false</isCollection>
        <isInput>false</isInput>
        <isOutput>false</isOutput>
        <objectType>Case</objectType>
    </variables>
    <variables>
        <description>the new version of the record in the DML operation</description>
        <name>record</name>
        <dataType>SObject</dataType>
        <isCollection>false</isCollection>
        <isInput>true</isInput>
        <isOutput>true</isOutput>
        <objectType>Case</objectType>
    </variables>
    <variables>
        <name>recordPrior</name>
        <dataType>SObject</dataType>
        <isCollection>false</isCollection>
        <isInput>true</isInput>
        <isOutput>false</isOutput>
        <objectType>Case</objectType>
    </variables>
</Flow>
`)
	writeFile(t, filepath.Join(root, base, "classes", name+".cls"), class)
	writeFile(t, filepath.Join(root, base, "classes", name+".cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>\n")
	run := Run(loadTestIndex(t, root), Options{})
	for _, suite := range run.Suites {
		for _, c := range suite.Cases {
			if c.Problem != nil {
				return invocableFlowIDRE.ReplaceAllString(strings.TrimSpace(strings.TrimPrefix(c.Problem.Message, "Assertion Failed: ")), "<ID>")
			}
		}
	}
	return ""
}

// Native rows: scratch org, API 62.0. Invocable.Action runs an autolaunched
// flow over unsaved Case records: RecordType.DeveloperName reads the record
// type from RecordTypeId, a rule without a connector ends the flow, Contains is
// false for a null side, and AddItem on a multi-select appends "; item".
func TestInvocableFlowActionCaseRecordRows(t *testing.T) {
	const want = `FLOW|A-noRT|success=true|errs=|out={Sub__c=Uno,Subject=a,}|inputAfter={Sub__c=Uno,Subject=a,}
FLOW|B-RT-reg-null|success=true|errs=|out={RecordTypeId=<ID>,Reg__c=Uno,Sub__c=Uno,Subject=b,}|inputAfter={RecordTypeId=<ID>,Sub__c=Uno,Subject=b,}
FLOW|C-RT-reg-has|success=true|errs=|out={RecordTypeId=<ID>,Reg__c=Dos; Uno,Sub__c=Uno,Subject=c,}|inputAfter={RecordTypeId=<ID>,Reg__c=Dos,Sub__c=Uno,Subject=c,}
FLOW|D-RT-reg-contains|success=true|errs=|out={RecordTypeId=<ID>,Reg__c=Uno;Dos,Sub__c=Uno,Subject=d,}|inputAfter={RecordTypeId=<ID>,Reg__c=Uno;Dos,Sub__c=Uno,Subject=d,}
FLOW|E-RT-sub-null|success=true|errs=|out={RecordTypeId=<ID>,Reg__c=,Subject=e,}|inputAfter={RecordTypeId=<ID>,Subject=e,}
FLOW|E2-RT-reg-has-sub-null|success=true|errs=|out={RecordTypeId=<ID>,Reg__c=Uno,Subject=e2,}|inputAfter={RecordTypeId=<ID>,Reg__c=Uno,Subject=e2,}
FLOW|F-RT-prior|success=true|errs=|out={RecordTypeId=<ID>,Reg__c=Dos,Sub__c=Dos,Subject=f,}|inputAfter={RecordTypeId=<ID>,Sub__c=Dos,Subject=f,}
FLOW|G-reg-Uno-sub-Uno|success=true|errs=|out={RecordTypeId=<ID>,Reg__c=Uno,Sub__c=Uno,Subject=g,}|inputAfter={RecordTypeId=<ID>,Reg__c=Uno,Sub__c=Uno,Subject=g,}
FLOW|H-RT-reg-Tres-Dos|success=true|errs=|out={RecordTypeId=<ID>,Reg__c=Tres;Dos; Uno,Sub__c=Uno,Subject=h,}|inputAfter={RecordTypeId=<ID>,Reg__c=Tres;Dos,Sub__c=Uno,Subject=h,}
FLOW|K-substr|success=true|errs=|out={RecordTypeId=<ID>,Reg__c=Dos;Tres,Sub__c=Dos,Subject=k,}|inputAfter={RecordTypeId=<ID>,Reg__c=Dos;Tres,Sub__c=Dos,Subject=k,}`
	if got := runInvocableFlowProbe(t, `@isTest
public class FlowProbe {
  static String o='';
  static void flowRun(String label, Case c, Case prior) {
    try {
      Invocable.Action a = Invocable.Action.createCustomAction('flow','IndFlow');
      Map<String,Object> m = new Map<String,Object>{'record'=>c};
      if (prior != null) m.put('recordPrior', prior);
      a.setInvocations(new List<Map<String,Object>>{m});
      List<Invocable.Action.Result> rs = a.invoke();
      Invocable.Action.Result r = rs[0];
      String errs='';
      for (Invocable.Action.Error e : r.getErrors()) errs += e.getCode()+':'+e.getMessage()+';';
      Object out = r.getOutputParameters().get('record');
      String s = 'null';
      if (out != null) { SObject so=(SObject)out; Map<String,Object> pm = so.getPopulatedFieldsAsMap(); List<String> ks = new List<String>(pm.keySet()); ks.sort(); s=''; for (String k : ks) s += k+'='+pm.get(k)+','; }
      Map<String,Object> im = c == null ? null : c.getPopulatedFieldsAsMap();
      String ins = ''; if (im != null) { List<String> ik = new List<String>(im.keySet()); ik.sort(); for (String k : ik) ins += k+'='+im.get(k)+','; }
      o += '\nFLOW|'+label+'|success='+r.isSuccess()+'|errs='+errs+'|out={'+s+'}|inputAfter={'+ins+'}';
    } catch (Exception e) { o += '\nFLOW|'+label+'|EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  }
  @isTest static void run() {
    Id rt = Case.SObjectType.getDescribe().getRecordTypeInfosByDeveloperName().get('Sol').getRecordTypeId();
    flowRun('A-noRT', new Case(Subject='a', Sub__c='Uno'), null);
    flowRun('B-RT-reg-null', new Case(Subject='b', RecordTypeId=rt, Sub__c='Uno'), null);
    flowRun('C-RT-reg-has', new Case(Subject='c', RecordTypeId=rt, Sub__c='Uno', Reg__c='Dos'), null);
    flowRun('D-RT-reg-contains', new Case(Subject='d', RecordTypeId=rt, Sub__c='Uno', Reg__c='Uno;Dos'), null);
    flowRun('E-RT-sub-null', new Case(Subject='e', RecordTypeId=rt), null);
    flowRun('E2-RT-reg-has-sub-null', new Case(Subject='e2', RecordTypeId=rt, Reg__c='Uno'), null);
    flowRun('F-RT-prior', new Case(Subject='f', RecordTypeId=rt, Sub__c='Dos'), new Case(Subject='f'));
    flowRun('G-reg-Uno-sub-Uno', new Case(Subject='g', RecordTypeId=rt, Sub__c='Uno', Reg__c='Uno'), null);
    flowRun('H-RT-reg-Tres-Dos', new Case(Subject='h', RecordTypeId=rt, Sub__c='Uno', Reg__c='Tres;Dos'), null);
    flowRun('K-substr', new Case(Subject='k', RecordTypeId=rt, Sub__c='Dos', Reg__c='Dos;Tres'), null);
    System.assert(false, o);
  }
}
`, "FlowProbe"); got != strings.TrimSpace(want) {
		t.Fatalf("native Invocable.Action Case rows differ\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestInvocableFlowActionContainsDecisionRows(t *testing.T) {
	const want = `CT|null-null|ok=true|subject=DEFAULT
CT|Uno-null|ok=true|subject=DEFAULT
CT|null-Uno|ok=true|subject=DEFAULT
CT|Uno-Uno|ok=true|subject=s
CT|Uno-Dos|ok=true|subject=DEFAULT
CT|UnoDos-Dos|ok=true|subject=s`
	if got := runInvocableFlowProbe(t, `@isTest
public class FlowProbe2 {
  static String o='';
  static void run1(String label, Case c) {
    try {
      Invocable.Action a = Invocable.Action.createCustomAction('flow','IndFlow2');
      a.setInvocations(new List<Map<String,Object>>{new Map<String,Object>{'record'=>c}});
      Invocable.Action.Result r = a.invoke()[0];
      SObject so = (SObject)r.getOutputParameters().get('record');
      o += '\nCT|'+label+'|ok='+r.isSuccess()+'|subject='+so.get('Subject');
    } catch (Exception e) { o += '\nCT|'+label+'|EXC|'+e.getMessage(); }
  }
  @isTest static void run() {
    Id rt = Case.SObjectType.getDescribe().getRecordTypeInfosByDeveloperName().get('Sol').getRecordTypeId();
    run1('null-null', new Case(Subject='s', RecordTypeId=rt));
    run1('Uno-null', new Case(Subject='s', RecordTypeId=rt, Reg__c='Uno'));
    run1('null-Uno', new Case(Subject='s', RecordTypeId=rt, Sub__c='Uno'));
    run1('Uno-Uno', new Case(Subject='s', RecordTypeId=rt, Sub__c='Uno', Reg__c='Uno'));
    run1('Uno-Dos', new Case(Subject='s', RecordTypeId=rt, Sub__c='Dos', Reg__c='Uno'));
    run1('UnoDos-Dos', new Case(Subject='s', RecordTypeId=rt, Sub__c='Dos', Reg__c='Uno;Dos'));
    System.assert(false, o);
  }
}
`, "FlowProbe2"); got != strings.TrimSpace(want) {
		t.Fatalf("native Contains decision rows differ\ngot:\n%s\nwant:\n%s", got, want)
	}
}
