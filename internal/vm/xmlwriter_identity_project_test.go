package vm_test

import (
	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
	"os"
	"path/filepath"
	"testing"
)

// SF201 admits both XML writer spellings at API65.
func TestXMLWriterIdentityAPI65(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	root := t.TempDir()
	files := map[string]string{
		"sfdx-project.json": `{"namespace":"","sourceApiVersion":"65.0","packageDirectories":[{"path":"force-app","default":true}]}`,
		"force-app/main/default/classes/GladeXmlWriter65Proof.cls": `@IsTest private class GladeXmlWriter65Proof {
 @IsTest static void originalCasingExportsEncodedContent() {
  Xmlstreamwriter out=new Xmlstreamwriter();
  out.writeStartDocument(null,'1.0');
  out.writeStartElement(null,'export',null);
  out.writeStartElement(null,'name',null);
  out.writeCharacters(EncodingUtil.urlEncode('A B&C','UTF-8'));
  out.writeEndElement();out.writeEndElement();out.writeEndDocument();
  String result=out.getXmlString();out.close();
  System.assert(result.contains('<name>A+B%26C</name>'),result);
  Dom.Document doc=new Dom.Document();doc.load(result);
  System.assertEquals('export',doc.getRootElement().getName());
  System.assertEquals('A+B%26C',doc.getRootElement().getChildElement('name',null).getText());
 }
 @IsTest static void canonicalCasingEscapesText() {
  XmlStreamWriter out=new XmlStreamWriter();
  out.writeStartDocument(null,'1.0');out.writeStartElement(null,'value',null);
  out.writeCharacters('<tag>&');out.writeEndElement();out.writeEndDocument();
  String result=out.getXmlString();out.close();
  System.assert(result.contains('&lt;tag&gt;&amp;'),result);
  Dom.Document doc=new Dom.Document();doc.load(result);
  System.assertEquals('<tag>&',doc.getRootElement().getText());
 }
}
`,
		"force-app/main/default/classes/GladeXmlWriter65Proof.cls-meta.xml": `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>65.0</apiVersion><status>Active</status></ApexClass>
`,
	}
	for name, content := range files {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	sch, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	report := apextest.Run(typesys.Build(p, sch), apextest.Options{NoDiskCache: true})
	if got := report.Summary(); got.Total != 2 || got.Passed != 2 || got.Errors != 0 {
		for _, suite := range report.Suites {
			for _, c := range suite.Cases {
				if c.Problem != nil {
					t.Logf("%s: %s", c.MethodName, c.Problem.Message)
				}
			}
		}
		t.Fatalf("XML writer contract: %+v", got)
	}
}
