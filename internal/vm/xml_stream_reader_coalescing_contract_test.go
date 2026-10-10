package vm

import "testing"

// API67 XmlStreamReader.setCoalescing(true) returns text as one block up to
// the first end element or next start element. Retained primary guide
// apex/apex_classes_xml_XmlStream_reader.md SHA256
// 1f33f6473b035df31b470eb1497edeae1e6bbe4caa0a41e34f3f96a82bfdc7c7,
// lines 679-698; getText text/CDATA values at 490-525, next at 645-657,
// and isCharacters at 592-603. Catalog SHA256
// 9d96c8f2472acb64d9d2860ab22ebb1e526edccf137b659c2bca441b1ce86a7b,
// /documents/646/members/30,19,28,24. The false-mode control permits any
// chunk count and does not rely on CDATA isCharacters classification.
// No comment/PI/DTD, location, event integer, namespace, midstream setter,
// native/API-interval, child AC7 or whole-family qualification is asserted.
func TestExecXmlStreamReaderCoalescingContractAPI67(t *testing.T) {
	cases := []struct {
		name   string
		source string
	}{
		{name: "endBoundary", source: `XmlStreamReader reader=new XmlStreamReader('<root>a<![CDATA[b]]>c</root>');reader.setCoalescing(true);reader.nextTag();reader.next();System.assert(reader.isCharacters());System.assertEquals('abc',reader.getText());reader.next();System.assert(reader.isEndElement());System.assertEquals('root',reader.getLocalName());reader=null;`},
		{name: "startBoundary", source: `XmlStreamReader reader=new XmlStreamReader('<root>a<![CDATA[b]]>c<child/></root>');reader.setCoalescing(true);reader.nextTag();reader.next();System.assert(reader.isCharacters());System.assertEquals('abc',reader.getText());reader.next();System.assert(reader.isStartElement());System.assertEquals('child',reader.getLocalName());reader=null;`},
		{name: "falseTextControl", source: `XmlStreamReader reader=new XmlStreamReader('<root>a<![CDATA[b]]>c</root>');reader.setCoalescing(false);reader.nextTag();reader.next();String accumulatedText='';while(!reader.isEndElement()){accumulatedText+=reader.getText();reader.next();}System.assertEquals('abc',accumulatedText);reader=null;`},
		{name: "plainTrueControl", source: `XmlStreamReader reader=new XmlStreamReader('<root>abc</root>');reader.setCoalescing(true);reader.nextTag();reader.next();System.assert(reader.isCharacters());System.assertEquals('abc',reader.getText());reader.next();System.assert(reader.isEndElement());System.assertEquals('root',reader.getLocalName());reader=null;`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(tc.source, CompileOptions{APIVersion: "67.0"})
			if err != nil {
				t.Fatal(err)
			}
			if program.APIVersion != "67.0" {
				t.Fatalf("compiled API version = %q, want 67.0", program.APIVersion)
			}
			if _, err := New(nil).Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
