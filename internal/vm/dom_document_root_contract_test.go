package vm

import "testing"

// Native captures cover repeated root creation and the initial-root control at API67.
// Retained Document SHA256
// cdc4d32c457c57a878fbb0f0ed738f725fa9ed8133407c38bc03d6a1cd6f87ca,
// line 80; catalog SHA256
// 9d96c8f2472acb64d9d2860ab22ebb1e526edccf137b659c2bca441b1ce86a7b,
// /documents/648/members/1. XmlNode.getName is supported by SHA256
// aad83601cc300ba9101f67a458f265e3148d0fd5bb9a3ff7d307f1567049fea6,
// lines 306-312. Rejection checks only execution-error existence; exception
// identity, catchability, state, phase, native and API-interval parity are unqualified.
func TestExecDomDocumentRejectsRepeatedRootCreationAPI67(t *testing.T) {
	cases := []struct {
		name      string
		source    string
		wantError bool
	}{
		{name: "repeatedRootCreation", source: `Dom.Document doc=new Dom.Document();doc.createRootElement('first',null,null);doc.createRootElement('second',null,null);doc=null;`, wantError: true},
		{name: "initialRootCreationControl", source: `Dom.Document doc=new Dom.Document();System.assertEquals(null,doc.getRootElement());doc.createRootElement('first',null,null);System.assertEquals('first',doc.getRootElement().getName());doc=null;`, wantError: false},
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
			_, err = New(nil).Execute(program)
			if (err != nil) != tc.wantError {
				t.Fatalf("execution error = %v, want error existence %v", err, tc.wantError)
			}
		})
	}
}
