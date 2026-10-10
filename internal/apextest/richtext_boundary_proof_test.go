package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// SF209 admits the complete API62/project40 tag and link boundary fixture.
func TestRichTextBoundaryAPI62Project40Proof(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace": "", "sourceApiVersion": "40.0", "packageDirectories": [{"path": "force-app", "default": true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeRichBoundary62Assertions.cls"), `@IsTest private class GladeRichBoundary62Assertions {
 @IsTest static void assertTagAndLinkBoundary() {
  List<String> inputs=new List<String>{'<strong>strong text</strong>','<em>emphasis</em>','<ol><li>one</li><li>two</li></ol>','<table><tbody><tr><td>cell</td></tr></tbody></table>','<a href="http://example.invalid/path">http</a>','<a href="mailto:owned@example.invalid">mail</a>','<a href="/owned/path">relative</a>','<a href="https://example.invalid/path">https</a>','<a href="javascript:alert(1)">unsafe</a>'};
  List<String> rich=new List<String>{'<strong>strong text</strong>','<em>emphasis</em>','<ol><li>one</li><li>two</li></ol>','<table><tbody><tr><td colspan="1" rowspan="1">cell</td></tr></tbody></table>','<a href="http://example.invalid/path" target="_blank">http</a>','<a href="mailto:owned@example.invalid" target="_blank">mail</a>','<a href="/owned/path" target="_blank">relative</a>','<a href="https://example.invalid/path" target="_blank">https</a>','<a href="" target="_blank">unsafe</a>'};
  List<String> plain=new List<String>{'<strong>strong text</strong>','<em>emphasis</em>','<ol><li>one</li><li>two</li></ol>','<table><tbody><tr><td>cell</td></tr></tbody></table>','<a href="http://example.invalid/path">http</a>','<a href="mailto:owned@example.invalid">mail</a>','<a href="/owned/path">relative</a>','<a href="https://example.invalid/path">https</a>','<a href="javascript:alert(1)">unsafe</a>'};
  for(Integer i=0;i<inputs.size();i++){
   GladeRichBoundary62Assert__c row=new GladeRichBoundary62Assert__c(Description__c=inputs[i],Plain__c=inputs[i]);insert row;
   GladeRichBoundary62Assert__c saved=[SELECT Description__c,Plain__c FROM GladeRichBoundary62Assert__c WHERE Id=:row.Id];
   System.assertEquals(inputs[i],row.Description__c,'caller rich '+i);
   System.assertEquals(inputs[i],row.Plain__c,'caller plain '+i);
   System.assertEquals(rich[i],saved.Description__c,'stored rich '+i);
   System.assertEquals(plain[i],saved.Plain__c,'stored plain '+i);
  }
 }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeRichBoundary62Assertions.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/GladeRichBoundary62Assert__c/GladeRichBoundary62Assert__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Rich Text Proof</label><pluralLabel>Rich Text Proofs</pluralLabel><nameField><label>Name</label><type>AutoNumber</type><displayFormat>R-{0000}</displayFormat></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel></CustomObject>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/GladeRichBoundary62Assert__c/fields/Description__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Description__c</fullName><label>Description</label><length>32000</length><type>Html</type><visibleLines>10</visibleLines></CustomField>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/GladeRichBoundary62Assert__c/fields/Plain__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Plain__c</fullName><label>Plain</label><length>32000</length><type>LongTextArea</type><visibleLines>10</visibleLines></CustomField>
`)
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 || got.Errors != 0 {
		data, _ := json.Marshal(run)
		t.Fatalf("SF209 boundary: %s", data)
	}
}
