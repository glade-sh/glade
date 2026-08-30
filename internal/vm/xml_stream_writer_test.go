package vm

import "testing"

func TestExecXmlStreamWriterScopesNamespaceDeclarations(t *testing.T) {
	program, err := CompileAnonymous(`
XmlStreamWriter writer = new XmlStreamWriter();
writer.writeStartDocument('UTF-8', '1.0');
writer.setDefaultNamespace('urn:base');
writer.writeStartElement('', 'root', 'urn:base');
writer.writeDefaultNamespace('urn:base');
writer.writeNamespace('x', 'urn:x');
writer.writeAttribute('x', 'urn:x', 'id', 'A&B');
writer.writeComment('note');
writer.writeProcessingInstruction('pi', 'go');
writer.writeCData('<raw>');
writer.writeStartElement('x', 'child', 'urn:x');
writer.writeCharacters('Tom & Sue');
writer.writeEndElement();
writer.writeEmptyElement('', 'leaf', 'urn:base');
writer.writeEndElement();
writer.writeEndDocument();
System.assertEquals('<?xml version="1.0" encoding="UTF-8"?><root xmlns="urn:base" xmlns:x="urn:x" x:id="A&amp;B"><!--note--><?pi go?><![CDATA[<raw>]]><x:child>Tom &amp; Sue</x:child><leaf/></root>', writer.getXmlString());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecXmlStreamWriterToStringUsesSystemDisplay(t *testing.T) {
	program, err := CompileAnonymous(`
XmlStreamWriter writer = new XmlStreamWriter();
System.assertEquals('System.XmlStreamWriter[]', writer.toString());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}
