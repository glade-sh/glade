package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// SF203 admits these API62/project40 rich-text persistence values.
func TestRichTextPersistenceAPI62Project40Proof(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"","sourceApiVersion":"40.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeRichMatrix62Assertions.cls"), `@IsTest private class GladeRichMatrix62Assertions {
 @IsTest static void assertInsertMatrix() {
  List<String> inputs=new List<String>{'Plain café & text','  first\nsecond  ','&lt;b&gt;encoded&lt;/b&gt; &amp; &#169;','<p>Hello <b>bold</b> <i>italic</i><br/>end</p>','<ul><li>one</li><li>two</li></ul>','before<script>alert(1)</script>after','<p onclick="alert(1)">text</p><img src=x onerror=alert(document.cookie)>','<img src="https://example.invalid/image.png" alt="Owned" width="10">','<a href="https://example.invalid/path" title="Owned">safe</a><a href="javascript:alert(1)">bad</a>','<a href="java&#115;cript:alert(1)">encoded</a>','<span style="color:red;font-weight:bold" class="owned" id="owned">styled</span>','<DIV OnMouseOver="alert(1)"><custom-owned>text</custom-owned></DIV>','<p>one<b>two</p>three<!-- comment -->'};
  List<String> rich=new List<String>{'Plain café &amp; text','first\nsecond','&lt;b&gt;encoded&lt;/b&gt; &amp; &copy;','<p>Hello <b>bold</b> <i>italic</i><br>end</p>','<ul><li>one</li><li>two</li></ul>','beforeafter','<p>text</p><img src=""></img>','<img src="https://example.invalid/image.png" alt="Owned" width="10"></img>','<a href="https://example.invalid/path" title="Owned" target="_blank">safe</a><a href="" target="_blank">bad</a>','<a href="" target="_blank">encoded</a>','<span class="owned" id="owned" style="color: red; font-weight: bold;">styled</span>','<div>text</div>','<p>one<b>two</b></p><b>three</b>'};
  List<String> plain=new List<String>{'Plain café & text','first\nsecond','&lt;b&gt;encoded&lt;/b&gt; &amp; &#169;','<p>Hello <b>bold</b> <i>italic</i><br/>end</p>','<ul><li>one</li><li>two</li></ul>','before<script>alert(1)</script>after','<p onclick="alert(1)">text</p><img src=x onerror=alert(document.cookie)>','<img src="https://example.invalid/image.png" alt="Owned" width="10">','<a href="https://example.invalid/path" title="Owned">safe</a><a href="javascript:alert(1)">bad</a>','<a href="java&#115;cript:alert(1)">encoded</a>','<span style="color:red;font-weight:bold" class="owned" id="owned">styled</span>','<DIV OnMouseOver="alert(1)"><custom-owned>text</custom-owned></DIV>','<p>one<b>two</p>three<!-- comment -->'};
  for(Integer i=0;i<inputs.size();i++) {
   GladeRichMatrix62Assert__c row=new GladeRichMatrix62Assert__c(Description__c=inputs[i],Plain__c=inputs[i]);insert row;
   GladeRichMatrix62Assert__c saved=[SELECT Description__c,Plain__c FROM GladeRichMatrix62Assert__c WHERE Id=:row.Id];
   System.assertEquals(inputs[i],row.Description__c,'caller '+i);
   System.assertEquals(rich[i],saved.Description__c,'rich '+i);
   System.assertEquals(plain[i],saved.Plain__c,'plain '+i);
  }
 }
 @IsTest static void assertUpdateAndRollback() {
  GladeRichMatrix62Assert__c row=new GladeRichMatrix62Assert__c(Description__c='<b>initial</b>',Plain__c='<b>initial</b>');insert row;
  Savepoint point=Database.setSavepoint();
  String changed='before<script>alert(1)</script><i onclick="alert(2)">after</i>';
  row.Description__c=changed;row.Plain__c=changed;update row;
  GladeRichMatrix62Assert__c updated=[SELECT Description__c,Plain__c FROM GladeRichMatrix62Assert__c WHERE Id=:row.Id];
  System.assertEquals(changed,row.Description__c);
  System.assertEquals('before<i>after</i>',updated.Description__c);
  System.assertEquals(changed,updated.Plain__c);
  Database.rollback(point);
  GladeRichMatrix62Assert__c restored=[SELECT Description__c,Plain__c FROM GladeRichMatrix62Assert__c WHERE Id=:row.Id];
  System.assertEquals('<b>initial</b>',restored.Description__c);
  System.assertEquals('<b>initial</b>',restored.Plain__c);
 }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeRichMatrix62Assertions.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/GladeRichMatrix62Assert__c/GladeRichMatrix62Assert__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Rich Text Proof</label><pluralLabel>Rich Text Proofs</pluralLabel><nameField><label>Name</label><type>AutoNumber</type><displayFormat>R-{0000}</displayFormat></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/GladeRichMatrix62Assert__c/fields/Description__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Description__c</fullName><label>Description</label><length>32000</length><type>Html</type><visibleLines>10</visibleLines></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/GladeRichMatrix62Assert__c/fields/Plain__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Plain__c</fullName><label>Plain</label><length>32000</length><type>LongTextArea</type><visibleLines>10</visibleLines></CustomField>`)
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 || got.Failed != 0 || got.Errors != 0 {
		data, _ := json.Marshal(run)
		t.Fatalf("SF203 rich-text persistence proof: %s", data)
	}
}
