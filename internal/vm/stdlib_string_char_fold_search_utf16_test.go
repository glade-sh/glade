package vm

import "testing"

func TestExecStringCharAndIgnoreCaseSearchUseUTF16PositionsAPI67(t *testing.T) {
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

	t.Run("indexOfChar/controls", func(t *testing.T) {
		run(t, `
String ascii = 'abca';
String bmp = 'aΩb';
System.assertEquals(0, ascii.indexOfChar(97));
System.assertEquals(1, bmp.indexOfChar(937));
System.assertEquals(-1, ascii.indexOfChar(122));
`)
	})
	t.Run("indexOfChar/supplementaryPrefix", func(t *testing.T) {
		run(t, `
String text = 'a😀b';
System.assertEquals(4, text.length());
System.assertEquals(3, text.indexOfChar(98), 'indexOfChar result follows the UTF-16 position after the supplementary character');
`)
	})

	t.Run("lastIndexOfChar/controls", func(t *testing.T) {
		run(t, `
String ascii = 'abca';
String bmp = 'aΩb';
System.assertEquals(3, ascii.lastIndexOfChar(97));
System.assertEquals(1, bmp.lastIndexOfChar(937));
System.assertEquals(-1, ascii.lastIndexOfChar(122));
`)
	})
	t.Run("lastIndexOfChar/supplementaryPrefix", func(t *testing.T) {
		run(t, `
String text = 'a😀b';
System.assertEquals(4, text.length());
System.assertEquals(3, text.lastIndexOfChar(98), 'lastIndexOfChar result follows the UTF-16 position after the supplementary character');
`)
	})

	t.Run("indexOfIgnoreCase/controls", func(t *testing.T) {
		run(t, `
String ascii = 'aB';
String bmp = 'aΩB';
System.assertEquals(1, ascii.indexOfIgnoreCase('b'));
System.assertEquals(2, bmp.indexOfIgnoreCase('b'));
System.assertEquals(-1, ascii.indexOfIgnoreCase('z'));
`)
	})
	t.Run("indexOfIgnoreCase/supplementaryPrefix", func(t *testing.T) {
		run(t, `
String text = 'a😀B';
System.assertEquals(4, text.length());
System.assertEquals(3, text.indexOfIgnoreCase('b'), 'indexOfIgnoreCase result follows the UTF-16 position after the supplementary character');
`)
	})

	t.Run("lastIndexOfIgnoreCase/controls", func(t *testing.T) {
		run(t, `
String ascii = 'aBab';
String bmp = 'aΩB';
System.assertEquals(3, ascii.lastIndexOfIgnoreCase('b'));
System.assertEquals(2, bmp.lastIndexOfIgnoreCase('b'));
System.assertEquals(-1, ascii.lastIndexOfIgnoreCase('z'));
`)
	})
	t.Run("lastIndexOfIgnoreCase/supplementaryPrefix", func(t *testing.T) {
		run(t, `
String text = 'a😀B';
System.assertEquals(4, text.length());
System.assertEquals(3, text.lastIndexOfIgnoreCase('b'), 'lastIndexOfIgnoreCase result follows the UTF-16 position after the supplementary character');
`)
	})
}

func TestStringCharAndIgnoreCaseSearchRejectsWrongArgumentTypes(t *testing.T) {
	for _, tc := range []struct {
		name string
		arg  Value
	}{
		{name: "indexOfChar", arg: String("b")},
		{name: "lastIndexOfChar", arg: String("b")},
		{name: "indexOfIgnoreCase", arg: Bool(true)},
		{name: "lastIndexOfIgnoreCase", arg: Bool(true)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, handled, err := callStringMember(String("abc"), tc.name, []Value{tc.arg})
			if !handled || err == nil {
				t.Fatalf("invalid argument accepted: handled=%t err=%v", handled, err)
			}
		})
	}
}
