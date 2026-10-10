package vm

import "testing"

func TestXMLWriterIdentityPreservesUserShadowAndExplicitSystem(t *testing.T) {
	ctor, err := CompileAnonymous(`this.marker = 'user';`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:         "XmlStreamWriter",
		Fields:       map[string]Field{"marker": {Name: "marker", Type: "String", Access: "public"}},
		Constructors: []Method{{Name: "XmlStreamWriter", ClassName: "XmlStreamWriter", IsConstructor: true, Program: ctor}},
	}); err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Xmlstreamwriter local = new Xmlstreamwriter();
System.assertEquals('user', local.marker);
System.Xmlstreamwriter platform = new System.Xmlstreamwriter();
platform.writeStartElement(null, 'value', null);
platform.writeCharacters('safe & text');
platform.writeEndElement();
System.assert(platform.getXmlString().contains('safe &amp; text'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestXMLWriterIdentityRetainsConstructorValidation(t *testing.T) {
	for _, source := range []string{`new Xmlstreamwriter('extra');`, `new System.Xmlstreamwriter(extra = 'value');`} {
		program, err := CompileAnonymous(source)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := New(nil).Execute(program); err == nil {
			t.Fatalf("invalid writer constructor accepted: %s", source)
		}
	}
}

func TestXMLWriterIdentityRejectsInvalidNullableArgumentTypes(t *testing.T) {
	for _, call := range []string{
		`writer.writeStartDocument(12, '1.0');`,
		`writer.writeStartDocument(null, 12);`,
		`writer.writeStartElement(12, 'value', null);`,
		`writer.writeStartElement(null, null, null);`,
		`writer.writeStartElement(null, 'value', 12);`,
	} {
		program, err := CompileAnonymous(`XmlStreamWriter writer = new XmlStreamWriter();` + call)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := New(nil).Execute(program); err == nil {
			t.Fatalf("invalid XML writer arguments accepted: %s", call)
		}
	}
}
