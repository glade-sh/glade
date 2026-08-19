package vm

import "testing"

func TestExecXmlStreamReaderNavigatesElementsTextAndAttributes(t *testing.T) {
	program, err := CompileAnonymous(`
XmlStreamReader reader = new XmlStreamReader('<root xmlns:p="urn:p" p:id="7" name="Acme"><child> text </child></root>');
System.assertEquals(XmlTag.START_DOCUMENT, reader.getEventType());
System.assert(reader.hasNext());

System.assertEquals(1, reader.next());
System.assert(reader.isStartElement());
System.assertEquals(XmlTag.START_ELEMENT, reader.getEventType());
System.assertEquals('root', reader.getLocalName());
System.assertEquals('Line: 1 Column: 44', reader.getLocation());
System.assertEquals(null, reader.getNamespace());
System.assertEquals('', reader.getPrefix());
try {
  reader.getPIData();
  System.assert(false);
} catch (XmlException e) {
  System.assert(e.getMessage().contains('Illegal State'));
}
try {
  reader.getPITarget();
  System.assert(false);
} catch (XmlException e) {
  System.assert(e.getMessage().contains('Illegal State'));
}
System.assertEquals(2, reader.getAttributeCount());
System.assertEquals('id', reader.getAttributeLocalName(0));
System.assertEquals('7', reader.getAttributeValueAt(0));
System.assertEquals('7', reader.getAttributeValue('urn:p', 'id'));
System.assertEquals(1, reader.getNamespaceCount());
System.assertEquals('p', reader.getNamespacePrefix(0));
System.assertEquals('urn:p', reader.getNamespaceURI('p'));

System.assertEquals(1, reader.nextTag());
System.assertEquals('child', reader.getLocalName());
System.assertEquals(4, reader.next());
System.assert(reader.isCharacters());
System.assertEquals(' text ', reader.getText());
System.assertEquals(2, reader.nextTag());
System.assert(reader.isEndElement());
System.assertEquals('child', reader.getLocalName());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecXmlStreamReaderReturnsDeclaredVersion(t *testing.T) {
	program, err := CompileAnonymous(`
XmlStreamReader reader = new XmlStreamReader('<?xml version="1.0"?><root/>');
System.assertEquals('1.0', reader.getVersion());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecXmlStreamReaderSafeDefaultsAndCaseInsensitiveMethods(t *testing.T) {
	program, err := CompileAnonymous(`
XmlStreamReader reader = new XmlStreamReader('<root/>');
reader.SETCOALESCING(true);
reader.setNamespaceAware(false);
System.assertEquals('', reader.getLocation());
System.assertEquals(null, reader.getVersion());
System.assertEquals(1, reader.NeXtTaG());
System.assertEquals(null, reader.getAttributeValue('', 'missing'));
System.assertEquals(null, reader.getAttributeLocalName(99));
System.assertEquals(null, reader.getNamespaceURI('missing'));
System.assertEquals(2, reader.nextTag());
System.assertEquals(8, reader.next());
System.assertEquals(false, reader.hasNext());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
