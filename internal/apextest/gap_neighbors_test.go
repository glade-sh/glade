package apextest

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// Neighbor shapes of the gap-capture fixes. Each shape is outside the captured
// rows and must keep its earlier behavior.

func gapNeighborProject(t *testing.T, api string, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
	for name, body := range files {
		writeFile(t, filepath.Join(root, "force-app/main/default", name), body)
	}
	return root
}

func gapClassMeta(api string) string {
	return `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>` + api + `</apiVersion><status>Active</status></ApexClass>`
}

func gapTriggerMeta(api string) string {
	return `<ApexTrigger xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>` + api + `</apiVersion><status>Active</status></ApexTrigger>`
}

func gapObject(label string, extra string) string {
	return `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>` + label + `</label><pluralLabel>` + label + `s</pluralLabel><nameField><label>Name</label><type>Text</type></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel>` + extra + `</CustomObject>`
}

// gapAdminProfile grants the administrator read access (edit when the field is
// listed in editable) to the named fields, as the native rows' permission set did.
func gapAdminProfile(objects []string, readable []string, editable []string) string {
	var b strings.Builder
	b.WriteString(`<Profile xmlns="http://soap.sforce.com/2006/04/metadata"><custom>false</custom>`)
	for _, field := range readable {
		fmt.Fprintf(&b, "<fieldPermissions><editable>false</editable><field>%s</field><readable>true</readable></fieldPermissions>", field)
	}
	for _, field := range editable {
		fmt.Fprintf(&b, "<fieldPermissions><editable>true</editable><field>%s</field><readable>true</readable></fieldPermissions>", field)
	}
	for _, object := range objects {
		fmt.Fprintf(&b, "<objectPermissions><allowCreate>true</allowCreate><allowDelete>true</allowDelete><allowEdit>true</allowEdit><allowRead>true</allowRead><modifyAllRecords>true</modifyAllRecords><object>%s</object><viewAllRecords>true</viewAllRecords></objectPermissions>", object)
	}
	b.WriteString("</Profile>")
	return b.String()
}

func gapRun(t *testing.T, root string, class string) {
	t.Helper()
	run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{class}, NoDiskCache: true, Parallelism: 1})
	for _, suite := range run.Suites {
		for _, entry := range suite.Cases {
			if entry.Problem != nil {
				t.Errorf("%s.%s: %s: %s", suite.Name, entry.MethodName, entry.Problem.Type, entry.Problem.Message)
			}
		}
	}
	if len(run.Suites) == 0 {
		t.Fatalf("no tests ran: %+v", run.Summary())
	}
}

// The roll-up is frozen only for the statement whose after-trigger is running:
// the next statement sees the earlier one's value, and a statement on a child
// without an after-trigger reads fresh values.
func TestGapRollupHoldScopeNeighbors(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := gapNeighborProject(t, api, map[string]string{
				"objects/HnP__c/HnP__c.object-meta.xml":        gapObject("Hn P", ""),
				"objects/HnP__c/fields/Cnt__c.field-meta.xml":  `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Cnt__c</fullName><label>Cnt</label><summaryForeignKey>HnC__c.P__c</summaryForeignKey><summaryOperation>count</summaryOperation><type>Summary</type></CustomField>`,
				"objects/HnC__c/HnC__c.object-meta.xml":        `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Hn C</label><pluralLabel>Hn Cs</pluralLabel><nameField><label>Name</label><type>Text</type></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ControlledByParent</sharingModel></CustomObject>`,
				"objects/HnC__c/fields/P__c.field-meta.xml":    `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>P__c</fullName><label>P</label><referenceTo>HnP__c</referenceTo><relationshipLabel>Hn Cs</relationshipLabel><relationshipName>HnCs</relationshipName><type>MasterDetail</type><reparentableMasterDetail>false</reparentableMasterDetail></CustomField>`,
				"objects/HnD__c/HnD__c.object-meta.xml":        `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Hn D</label><pluralLabel>Hn Ds</pluralLabel><nameField><label>Name</label><type>Text</type></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ControlledByParent</sharingModel></CustomObject>`,
				"objects/HnD__c/fields/P__c.field-meta.xml":    `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>P__c</fullName><label>P</label><referenceTo>HnP__c</referenceTo><relationshipLabel>Hn Ds</relationshipLabel><relationshipName>HnDs</relationshipName><type>MasterDetail</type><reparentableMasterDetail>false</reparentableMasterDetail></CustomField>`,
				"objects/HnP__c/fields/CntD__c.field-meta.xml": `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>CntD__c</fullName><label>CntD</label><summaryForeignKey>HnD__c.P__c</summaryForeignKey><summaryOperation>count</summaryOperation><type>Summary</type></CustomField>`,
				"classes/HnLog.cls":                            `public class HnLog { public static List<String> seen = new List<String>(); }`,
				"classes/HnLog.cls-meta.xml":                   gapClassMeta(api),
				"triggers/HnCTrg.trigger":                      `trigger HnCTrg on HnC__c (after insert, after delete) { for (HnP__c p : [SELECT Cnt__c FROM HnP__c WHERE Id = :Trigger.new[0].P__c]) { HnLog.seen.add(String.valueOf(p.Cnt__c)); } }`,
				"triggers/HnCTrg.trigger-meta.xml":             gapTriggerMeta(api),
				"triggers/HnDTrg.trigger":                      `trigger HnDTrg on HnD__c (before insert) { Integer n = 0; }`,
				"triggers/HnDTrg.trigger-meta.xml":             gapTriggerMeta(api),
				"classes/HnTest.cls": `@IsTest private class HnTest {
  @IsTest static void scope() {
    HnP__c p = new HnP__c(Name = 'p'); insert p;
    insert new HnC__c(Name = 'a', P__c = p.Id);
    insert new HnC__c(Name = 'b', P__c = p.Id);
    System.assertEquals('0,1', String.join(HnLog.seen, ','), 'each statement sees the earlier value');
    System.assertEquals('2', String.valueOf([SELECT Cnt__c FROM HnP__c WHERE Id = :p.Id].Cnt__c), 'fresh after the statement');
    // A child with only a before-trigger reads fresh values after its statement.
    insert new HnD__c(Name = 'd', P__c = p.Id);
    System.assertEquals('1', String.valueOf([SELECT CntD__c FROM HnP__c WHERE Id = :p.Id].CntD__c));
  }
}`,
				"classes/HnTest.cls-meta.xml":     gapClassMeta(api),
				"profiles/Admin.profile-meta.xml": gapAdminProfile([]string{"HnP__c", "HnC__c", "HnD__c"}, []string{"HnP__c.Cnt__c", "HnP__c.CntD__c"}, nil),
			})
			gapRun(t, root, "HnTest")
		})
	}
}

// A trigger whose nested DML inserts another child of the same parent reads the
// roll-up with that inner change (the earlier behavior); only parents no inner
// statement changed keep the frozen value. Not captured natively.
func TestGapRollupHoldDroppedByInnerStatement(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := gapNeighborProject(t, api, map[string]string{
				"objects/HiP__c/HiP__c.object-meta.xml":       gapObject("Hi P", ""),
				"objects/HiP__c/fields/Cnt__c.field-meta.xml": `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Cnt__c</fullName><label>Cnt</label><summaryForeignKey>HiC__c.P__c</summaryForeignKey><summaryOperation>count</summaryOperation><type>Summary</type></CustomField>`,
				"objects/HiC__c/HiC__c.object-meta.xml":       `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Hi C</label><pluralLabel>Hi Cs</pluralLabel><nameField><label>Name</label><type>Text</type></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ControlledByParent</sharingModel></CustomObject>`,
				"objects/HiC__c/fields/P__c.field-meta.xml":   `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>P__c</fullName><label>P</label><referenceTo>HiP__c</referenceTo><relationshipLabel>Hi Cs</relationshipLabel><relationshipName>HiCs</relationshipName><type>MasterDetail</type><reparentableMasterDetail>false</reparentableMasterDetail></CustomField>`,
				"classes/HiLog.cls":                           `public class HiLog { public static Boolean nested = false; public static String seen = ''; }`,
				"classes/HiLog.cls-meta.xml":                  gapClassMeta(api),
				"triggers/HiCTrg.trigger": `trigger HiCTrg on HiC__c (after insert) {
  if (!HiLog.nested) { HiLog.nested = true; insert new HiC__c(Name = 'inner', P__c = Trigger.new[0].P__c); }
  HiLog.seen += String.valueOf([SELECT Cnt__c FROM HiP__c WHERE Id = :Trigger.new[0].P__c].Cnt__c) + ',';
}`,
				"triggers/HiCTrg.trigger-meta.xml": gapTriggerMeta(api),
				"classes/HiTest.cls": `@IsTest private class HiTest { @IsTest static void inner() {
  HiP__c p = new HiP__c(Name = 'p'); insert p;
  insert new HiC__c(Name = 'a', P__c = p.Id);
  System.assertEquals('0,1,', HiLog.seen);
} }`,
				"classes/HiTest.cls-meta.xml":     gapClassMeta(api),
				"profiles/Admin.profile-meta.xml": gapAdminProfile([]string{"HiP__c", "HiC__c"}, []string{"HiP__c.Cnt__c"}, nil),
			})
			gapRun(t, root, "HiTest")
		})
	}
}

// An ordinary exception from a nested trigger stays catchable at every level;
// only the depth failure unwinds past the inner activations.
func TestGapTriggerOrdinaryExceptionStaysCatchableNeighbors(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := gapNeighborProject(t, api, map[string]string{
				"objects/GnA__c/GnA__c.object-meta.xml": gapObject("Gn A", ""),
				"objects/GnB__c/GnB__c.object-meta.xml": gapObject("Gn B", ""),
				"objects/GnC__c/GnC__c.object-meta.xml": gapObject("Gn C", ""),
				"classes/GnLog.cls":                     `public class GnLog { public static String log = ''; }`,
				"classes/GnLog.cls-meta.xml":            gapClassMeta(api),
				"triggers/GnATrg.trigger":               `trigger GnATrg on GnA__c (after update) { GnB__c b = [SELECT Id FROM GnB__c LIMIT 1]; try { update b; } catch (DmlException e) { GnLog.log += 'A'; } }`,
				"triggers/GnATrg.trigger-meta.xml":      gapTriggerMeta(api),
				"triggers/GnBTrg.trigger":               `trigger GnBTrg on GnB__c (after update) { GnC__c c = [SELECT Id FROM GnC__c LIMIT 1]; try { update c; } catch (DmlException e) { GnLog.log += 'B'; throw e; } }`,
				"triggers/GnBTrg.trigger-meta.xml":      gapTriggerMeta(api),
				"triggers/GnCTrg.trigger":               `trigger GnCTrg on GnC__c (after update) { throw new DmlException('plain'); }`,
				"triggers/GnCTrg.trigger-meta.xml":      gapTriggerMeta(api),
				"classes/GnTest.cls": `@IsTest private class GnTest {
  @IsTest static void ordinary() {
    GnA__c a = new GnA__c(Name = 'a'); insert a;
    insert new GnB__c(Name = 'b');
    insert new GnC__c(Name = 'c');
    update a;
    System.assertEquals('BA', GnLog.log, 'both catch blocks run for an ordinary exception');
  }
}`,
				"classes/GnTest.cls-meta.xml": gapClassMeta(api),
			})
			gapRun(t, root, "GnTest")
		})
	}
}

// Standard-object field maps and custom Text and Checkbox grouping keep their
// earlier behavior next to the custom-object changes.
func TestGapDescribeAndGroupNeighbors(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := gapNeighborProject(t, api, map[string]string{
				"objects/GdO__c/GdO__c.object-meta.xml":      gapObject("Gd O", ""),
				"objects/GdO__c/fields/Tx__c.field-meta.xml": `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Tx__c</fullName><label>Tx</label><length>20</length><type>Text</type></CustomField>`,
				"objects/GdO__c/fields/Ck__c.field-meta.xml": `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Ck__c</fullName><label>Ck</label><defaultValue>false</defaultValue><type>Checkbox</type></CustomField>`,
				"classes/GdTest.cls": `@IsTest private class GdTest {
  @IsTest static void neighbors() {
    insert new GdO__c(Name = 'o', Tx__c = 'x', Ck__c = true);
    List<AggregateResult> rs = [SELECT Tx__c t, COUNT(Id) c FROM GdO__c GROUP BY Tx__c];
    System.assertEquals(1, rs.size());
    rs = [SELECT Ck__c k, COUNT(Id) c FROM GdO__c GROUP BY Ck__c];
    System.assertEquals(1, rs.size());
    List<String> keys = new List<String>(Account.SObjectType.getDescribe().fields.getMap().keySet());
    System.assertEquals('accountnumber', keys[0], 'standard objects keep the sorted order');
    List<String> own = new List<String>(GdO__c.SObjectType.getDescribe().fields.getMap().keySet());
    System.assertEquals('id', own[0]);
    System.assertEquals('systemmodstamp', own[8]);
    System.assertEquals('ck__c', own[9]);
    System.assertEquals('tx__c', own[10]);
  }
}`,
				"classes/GdTest.cls-meta.xml":     gapClassMeta(api),
				"profiles/Admin.profile-meta.xml": gapAdminProfile([]string{"GdO__c"}, nil, []string{"GdO__c.Tx__c", "GdO__c.Ck__c"}),
			})
			gapRun(t, root, "GdTest")
		})
	}
}

// A trigger that reaches its DML through a handler class must see the depth
// failure the same way: catch, finally and partial Database calls work only at
// the outermost trigger, and the caller gets the wrapped DmlException.
func TestGapTriggerDepthThroughHandlerClass(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := gapNeighborProject(t, api, map[string]string{
				"objects/HdA__c/HdA__c.object-meta.xml": gapObject("Hd A", ""),
				"objects/HdB__c/HdB__c.object-meta.xml": gapObject("Hd B", ""),
				"classes/HdCtl.cls": `public class HdCtl { public static Integer depth = 0; public static String mode = 'plain'; public static String log = '';
  public static void bounce(SObject o) {
    depth++; Integer d = depth;
    SObject next = o instanceof HdA__c ? (SObject)[SELECT Id FROM HdB__c LIMIT 1] : (SObject)[SELECT Id FROM HdA__c LIMIT 1];
    next.put('Name', 'n' + d);
    if (mode == 'catch') { try { update next; } catch (DmlException e) { log += d + 'C,'; } finally { log += d + 'F,'; } }
    else if (mode == 'partial') { Database.SaveResult r = Database.update(next, false); if (!r.isSuccess()) { log += d + 'P,'; } }
    else { update next; }
  } }`,
				"classes/HdCtl.cls-meta.xml":       gapClassMeta(api),
				"triggers/HdATrg.trigger":          `trigger HdATrg on HdA__c (after update) { HdCtl.bounce(Trigger.new[0]); }`,
				"triggers/HdATrg.trigger-meta.xml": gapTriggerMeta(api),
				"triggers/HdBTrg.trigger":          `trigger HdBTrg on HdB__c (after update) { HdCtl.bounce(Trigger.new[0]); }`,
				"triggers/HdBTrg.trigger-meta.xml": gapTriggerMeta(api),
				"classes/HdTest.cls": `@IsTest private class HdTest {
  static HdA__c seed() { HdA__c a = new HdA__c(Name = 'a'); insert a; insert new HdB__c(Name = 'b'); HdCtl.depth = 0; HdCtl.log = ''; return a; }
  @IsTest static void plain() {
    HdA__c a = seed(); HdCtl.mode = 'plain'; a.Name = 'x';
    try { update a; System.assert(false, 'no error'); } catch (DmlException e) { System.assert(e.getMessage().contains('maximum trigger depth exceeded'), e.getMessage()); }
    System.assertEquals(16, HdCtl.depth);
  }
  @IsTest static void catchAndFinally() {
    HdA__c a = seed(); HdCtl.mode = 'catch'; a.Name = 'x';
    update a;
    System.assertEquals('1C,1F,', HdCtl.log);
  }
  @IsTest static void innerPartial() {
    HdA__c a = seed(); HdCtl.mode = 'partial'; a.Name = 'x';
    update a;
    System.assertEquals('1P,', HdCtl.log);
  }
  @IsTest static void topLevelPartial() {
    HdA__c a = seed(); HdCtl.mode = 'plain'; a.Name = 'x';
    Database.SaveResult r = Database.update(a, false);
    System.assert(!r.isSuccess());
    System.assertEquals('CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY', String.valueOf(r.getErrors()[0].getStatusCode()));
  }
}`,
				"classes/HdTest.cls-meta.xml": gapClassMeta(api),
			})
			gapRun(t, root, "HdTest")
		})
	}
}
