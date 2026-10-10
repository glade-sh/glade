package vm

import "testing"

func TestExecStringCharAtCompleteCharacterUsesUTF16IndexAPI67(t *testing.T) {
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

	t.Run("ascii", func(t *testing.T) {
		run(t, `
String ascii = 'ABC';
System.assertEquals(3, ascii.length());
System.assertEquals(65, ascii.charAt(0));
System.assertEquals(67, ascii.charAt(2));
`)
	})

	t.Run("bmp", func(t *testing.T) {
		run(t, `
String bmp = 'aΩb';
System.assertEquals(3, bmp.length());
System.assertEquals(937, bmp.charAt(1));
System.assertEquals(98, bmp.charAt(2));
`)
	})

	t.Run("supplementarySuffix", func(t *testing.T) {
		run(t, `
String supplementary = 'a😀b';
System.assertEquals(4, supplementary.length());
System.assertEquals(98, supplementary.charAt(3), 'charAt uses the complete character UTF-16 index');
`)
	})
}

func TestStringCharAtPreservesArgumentAndBoundsErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		text string
		args []Value
	}{
		{name: "missing argument", text: "ABC"},
		{name: "wrong argument type", text: "ABC", args: []Value{String("1")}},
		{name: "extra argument", text: "ABC", args: []Value{Int(0), Int(1)}},
		{name: "negative index", text: "ABC", args: []Value{Int(-1)}},
		{name: "empty receiver", args: []Value{Int(0)}},
		{name: "index equals UTF-16 length", text: "a😀b", args: []Value{Int(4)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, handled, err := callStringMember(String(tc.text), "charAt", tc.args)
			if !handled || err == nil {
				t.Fatalf("charAt should reject invalid input: handled=%t err=%v", handled, err)
			}
		})
	}
}
