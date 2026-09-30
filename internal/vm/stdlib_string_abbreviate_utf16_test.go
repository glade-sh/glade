package vm

import "testing"

// Source: Summer '26 catalog SHA-256 9d96c8f2472acb64d9d2860ab22ebb1e526edccf137b659c2bca441b1ce86a7b,
// doc 2219 members /0 and /1 document abbreviate(maxWidth) and
// abbreviate(maxWidth, offset). Member /72 defines length as 16-bit Unicode
// characters and /98 anchors zero-based substring ranges. Applying those
// units to abbreviate width and offset is source-inferred, not stated verbatim.
func TestExecStringAbbreviateUsesUTF16WidthsAndOffsetsAPI67(t *testing.T) {
	run := func(t *testing.T, source string) {
		t.Helper()
		program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "67.0"})
		if err != nil {
			t.Fatal(err)
		}
		if program.APIVersion != "67.0" {
			t.Fatalf("compiled API version = %q, want 67.0", program.APIVersion)
		}
		if _, err := Execute(program, nil); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("oneArgumentASCIIControl", func(t *testing.T) {
		run(t, `
String result = 'abcdefghij'.abbreviate(9);
System.assert(result.equals('abcdef...'));
System.assertEquals(9, result.length());
`)
	})
	t.Run("oneArgumentBMPControl", func(t *testing.T) {
		run(t, `
String result = 'aΩbcdefghj'.abbreviate(9);
System.assert(result.equals('aΩbcde...'));
System.assertEquals(9, result.length());
`)
	})
	t.Run("oneArgumentSupplementaryWidth", func(t *testing.T) {
		run(t, `
String source = 'a😀bcdefgh';
String result = source.abbreviate(9);
System.assertEquals(10, source.length());
System.assert(result.equals('a😀bcd...'));
System.assertEquals(9, result.length());
`)
	})
	t.Run("offsetArgumentASCIIControl", func(t *testing.T) {
		run(t, `
String result = 'abcdefghijklmnopq'.abbreviate(10, 7);
System.assert(result.equals('...hijk...'));
System.assertEquals(10, result.length());
`)
	})
	t.Run("offsetArgumentBMPControl", func(t *testing.T) {
		run(t, `
String result = 'aΩbcdefghijklmnop'.abbreviate(10, 7);
System.assert(result.equals('...ghij...'));
System.assertEquals(10, result.length());
`)
	})
	t.Run("offsetArgumentSupplementaryPosition", func(t *testing.T) {
		run(t, `
String source = 'a😀bcdefghijklmno';
String result = source.abbreviate(10, 7);
System.assertEquals(17, source.length());
System.assert(result.equals('...fghi...'));
System.assertEquals(10, result.length());
`)
	})
}
