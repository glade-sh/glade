package vm

import "testing"

func TestExecStringSearchOneArgumentUsesUTF16PositionsAPI67(t *testing.T) {
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

	t.Run("indexOf", func(t *testing.T) {
		run(t, `
String supplementary = 'a😀b';
String ascii = 'abca';
System.assertEquals(0, ascii.indexOf('a'));
String bmp = 'aΩb';
System.assertEquals(2, bmp.indexOf('b'));
System.assertEquals(-1, supplementary.indexOf('z'));
System.assertEquals(3, supplementary.indexOf('b'), 'indexOf uses UTF-16 positions');
`)
	})

	t.Run("lastIndexOf", func(t *testing.T) {
		run(t, `
String supplementary = 'a😀b';
String ascii = 'abca';
System.assertEquals(3, ascii.lastIndexOf('a'));
String bmp = 'aΩb';
System.assertEquals(2, bmp.lastIndexOf('b'));
System.assertEquals(-1, supplementary.lastIndexOf('z'));
System.assertEquals(3, supplementary.lastIndexOf('b'), 'lastIndexOf uses UTF-16 positions');
`)
	})
}
