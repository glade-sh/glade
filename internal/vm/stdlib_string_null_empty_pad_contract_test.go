package vm

import "testing"

// API67 family observations supersede the earlier null-padding inference.
// Null padding throws; empty padding retains the existing space-padding controls.
func TestExecStringNullEmptyPadContractAPI67(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source string
	}{
		{
			name: "leftAsciiNull",
			source: `String text = 'xy';
String pad = null;
Boolean threw = false;
try { text.leftPad(4, pad); } catch (NullPointerException e) { threw = true; System.assertEquals('Argument cannot be null.', e.getMessage()); }
System.assert(threw);
`,
		},
		{
			name: "leftSupplementaryNull",
			source: `String text = '😀x';
String pad = null;
Boolean threw = false;
try { text.leftPad(5, pad); } catch (NullPointerException e) { threw = true; System.assertEquals('Argument cannot be null.', e.getMessage()); }
System.assert(threw);
`,
		},
		{
			name: "leftSupplementaryEmpty",
			source: `String text = '😀x';
String pad = '';
String actual = text.leftPad(5, pad);
System.assertEquals('  😀x', actual);
`,
		},
		{
			name: "leftAsciiEmpty",
			source: `String text = 'xy';
String pad = '';
String actual = text.leftPad(4, pad);
System.assertEquals('  xy', actual);
`,
		},
		{
			name: "rightAsciiNull",
			source: `String text = 'xy';
String pad = null;
Boolean threw = false;
try { text.rightPad(4, pad); } catch (NullPointerException e) { threw = true; System.assertEquals('Argument cannot be null.', e.getMessage()); }
System.assert(threw);
`,
		},
		{
			name: "rightSupplementaryNull",
			source: `String text = '😀x';
String pad = null;
Boolean threw = false;
try { text.rightPad(5, pad); } catch (NullPointerException e) { threw = true; System.assertEquals('Argument cannot be null.', e.getMessage()); }
System.assert(threw);
`,
		},
		{
			name: "rightSupplementaryEmpty",
			source: `String text = '😀x';
String pad = '';
String actual = text.rightPad(5, pad);
System.assertEquals('😀x  ', actual);
`,
		},
		{
			name: "rightAsciiEmpty",
			source: `String text = 'xy';
String pad = '';
String actual = text.rightPad(4, pad);
System.assertEquals('xy  ', actual);
`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			program, compileErr := CompileAnonymousWithOptions(tc.source, CompileOptions{APIVersion: "67.0"})
			if compileErr != nil {
				t.Fatalf("step=compile case=%s api=67.0 source=%q compileErr=%v", tc.name, tc.source, compileErr)
			}
			if program.APIVersion != "67.0" {
				t.Fatalf("step=program-api-guard case=%s api=67.0 source=%q got=%q want=67.0 error=compiled-api-mismatch", tc.name, tc.source, program.APIVersion)
			}
			machine := New(nil)
			if _, executeErr := machine.Execute(program); executeErr != nil {
				t.Fatalf("step=execute case=%s api=67.0 source=%q executeErr=%v", tc.name, tc.source, executeErr)
			}
		})
	}
}
