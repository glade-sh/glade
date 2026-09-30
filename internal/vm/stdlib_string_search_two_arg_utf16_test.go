package vm

import "testing"

func TestExecStringTwoArgumentSearchUsesUTF16CompleteCharacterIndexAPI67(t *testing.T) {
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

	t.Run("indexOfControls", func(t *testing.T) {
		run(t, `
String ascii = 'abcab';
String bmp = 'aΩb';
System.assertEquals(4, ascii.indexOf('b', 2));
System.assertEquals(2, bmp.indexOf('b', 2));
System.assertEquals(1, bmp.indexOf('Ω', 1));
System.assertEquals(-1, bmp.indexOf('z', 1));
`)
	})
	t.Run("indexOfSupplementarySuffix", func(t *testing.T) {
		run(t, `
String supplementary = 'a😀b';
System.assertEquals(4, supplementary.length());
System.assertEquals(3, supplementary.indexOf('b', 3), 'indexOf accepts the UTF-16 suffix position');
System.assertEquals(3, supplementary.indexOf('b', 0), 'indexOf returns a UTF-16 position');
System.assertEquals(-1, supplementary.indexOf('z', 3));
`)
	})
	t.Run("lastIndexOfControls", func(t *testing.T) {
		run(t, `
String ascii = 'abcab';
String bmp = 'aΩb';
System.assertEquals(1, ascii.lastIndexOf('b', 3));
System.assertEquals(2, bmp.lastIndexOf('b', 2));
System.assertEquals(1, bmp.lastIndexOf('Ω', 1));
System.assertEquals(-1, bmp.lastIndexOf('z', 2));
`)
	})
	t.Run("lastIndexOfSupplementarySuffix", func(t *testing.T) {
		run(t, `
String supplementary = 'a😀b';
System.assertEquals(4, supplementary.length());
System.assertEquals(3, supplementary.lastIndexOf('b', 3), 'lastIndexOf returns the UTF-16 suffix position');
System.assertEquals(-1, supplementary.lastIndexOf('b', 0));
System.assertEquals(-1, supplementary.lastIndexOf('z', 3));
`)
	})
}

func TestStringTwoArgumentSearchPreservesArgumentErrors(t *testing.T) {
	for _, method := range []string{"indexOf", "lastIndexOf"} {
		for _, tc := range []struct {
			name string
			args []Value
		}{
			{name: "missing needle"},
			{name: "wrong needle type", args: []Value{Int(65), Int(0)}},
			{name: "wrong index type", args: []Value{String("a"), String("0")}},
			{name: "extra argument", args: []Value{String("a"), Int(0), Int(1)}},
		} {
			t.Run(method+"/"+tc.name, func(t *testing.T) {
				_, handled, err := callStringMember(String("abc"), method, tc.args)
				if !handled || err == nil {
					t.Fatalf("%s should reject invalid input: handled=%t err=%v", method, handled, err)
				}
			})
		}
	}
}
