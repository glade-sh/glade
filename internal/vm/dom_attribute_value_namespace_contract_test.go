package vm

import "testing"

// API67 retained source: Dom.Document cdc4d32c457c57a878fbb0f0ed738f725fa9ed8133407c38bc03d6a1cd6f87ca,
// XmlNode aad83601cc300ba9101f67a458f265e3148d0fd5bb9a3ff7d307f1567049fea6,
// DOM guide 14c38eda37118e42b410bc7531211d9d21ba85866d32733a23b09488ec7efbdc.
// The explicit QName example distinguishes raw attribute text, the local value,
// and its namespace. Index lookup has no attribute-order assumption. These are
// six source-qualified cases, not whole-DOM or native/API-interval parity.
func TestExecDomAttributeQNameValueGettersAPI67(t *testing.T) {
	cases := []struct {
		name, source string
	}{
		{"localQNameValue", `
Dom.Document doc = new Dom.Document();doc.load('<square dimension="2" ns1:example="ns2:test" xmlns:ns1="http://ns1" xmlns:ns2="http://ns2" />');Dom.XmlNode root=doc.getRootElement();System.assertEquals('test',root.getAttributeValue('example','http://ns1'));
`},
		{"QNameValueNamespace", `
Dom.Document doc = new Dom.Document();doc.load('<square dimension="2" ns1:example="ns2:test" xmlns:ns1="http://ns1" xmlns:ns2="http://ns2" />');Dom.XmlNode root=doc.getRootElement();System.assertEquals('http://ns2',root.getAttributeValueNs('example','http://ns1'));
`},
		{"rawQNameControl", `
Dom.Document doc = new Dom.Document();doc.load('<square dimension="2" ns1:example="ns2:test" xmlns:ns1="http://ns1" xmlns:ns2="http://ns2" />');Dom.XmlNode root=doc.getRootElement();System.assertEquals('ns2:test',root.getAttribute('example','http://ns1'));
`},
		{"unqualifiedControl", `
Dom.Document doc = new Dom.Document();doc.load('<square dimension="2" ns1:example="ns2:test" xmlns:ns1="http://ns1" xmlns:ns2="http://ns2" />');Dom.XmlNode root=doc.getRootElement();System.assertEquals('2',root.getAttributeValue('dimension',null));
`},
		{"namespaceIndexControl", `
Dom.Document doc = new Dom.Document();doc.load('<square dimension="2" ns1:example="ns2:test" xmlns:ns1="http://ns1" xmlns:ns2="http://ns2" />');Dom.XmlNode root=doc.getRootElement();System.assertEquals(2,root.getAttributeCount());Integer found=-1;for(Integer i=0;i<root.getAttributeCount();i++){if(root.getAttributeKeyAt(i).equals('example')){found=i;}}System.assert(found>=0);System.assertEquals('http://ns1',root.getAttributeKeyNsAt(found));
`},
		{"childNamespaceControl", `
Dom.Document doc=new Dom.Document();doc.load('<root xmlns:n="urn:n"><n:item>named</n:item><item>plain</item></root>');Dom.XmlNode root=doc.getRootElement();System.assertEquals('named',root.getChildElement('item','urn:n').getText());System.assertEquals('plain',root.getChildElement('item',null).getText());doc=null;root=null;
`},
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
			machine := New(nil)
			machine.EnableTestContext()
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Native QName controls Q001-Q015 are identical at
// API 62/67: bound prefixes resolve permissive suffixes, while 1ns stays raw.
func TestDomAttributeValuePartsMatchesNativeMalformedQNameText(t *testing.T) {
	node := newDomXmlNode("ELEMENT", "square", "", "")
	namespaces := typedMap("Map<String,String>")
	namespaces.Map[mapKey(String("ns2"))] = String("http://ns2")
	namespaces.Map[mapKey(String("1ns"))] = String("urn:invalid-prefix")
	node.Fields["namespaces"] = namespaces
	for _, tc := range []struct {
		rows, text, value string
		namespace         Value
	}{
		{"Q002/Q003", "ns2:two words", "two words", String("http://ns2")},
		{"Q005/Q006", "ns2:1name", "1name", String("http://ns2")},
		{"Q008/Q009", "ns2:two/name", "two/name", String("http://ns2")},
		{"Q011/Q012", "ns2:two:test", "two:test", String("http://ns2")},
		{"Q014/Q015", "1ns:test", "1ns:test", Null},
	} {
		t.Run(tc.rows, func(t *testing.T) {
			attr := domAttribute("example", tc.text, "http://ns1", "")
			value, namespace := domAttributeValueParts(node, attr)
			if value.Kind != ValueString || value.Text != tc.value || namespace.Kind != tc.namespace.Kind || namespace.Text != tc.namespace.Text {
				t.Fatalf("attribute %q: value=%v namespace=%v, want value=%q namespace=%v", tc.text, value, namespace, tc.value, tc.namespace)
			}
		})
	}
}
