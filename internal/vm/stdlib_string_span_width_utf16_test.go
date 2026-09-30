package vm

import "testing"

func TestExecStringSpanAndDefaultWidthsUseUTF16API67(t *testing.T) {
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

	t.Run("leftControls", func(t *testing.T) {
		run(t, `
String ascii = 'abc';
String bmp = 'aΩb';
System.assertEquals('ab', ascii.left(2));
System.assertEquals('aΩ', bmp.left(2));
`)
	})
	t.Run("leftSupplementaryPrefix", func(t *testing.T) {
		run(t, `
String text = 'a😀b';
System.assertEquals(4, text.length());
System.assertEquals('a😀', text.left(3));
`)
	})

	t.Run("rightControls", func(t *testing.T) {
		run(t, `
String ascii = 'abc';
String bmp = 'aΩb';
System.assertEquals('bc', ascii.right(2));
System.assertEquals('Ωb', bmp.right(2));
`)
	})
	t.Run("rightSupplementaryPrefix", func(t *testing.T) {
		run(t, `
String text = 'a😀b';
System.assertEquals(4, text.length());
System.assertEquals('😀b', text.right(3));
`)
	})

	t.Run("midControls", func(t *testing.T) {
		run(t, `
String ascii = 'abcd';
String bmp = 'aΩb';
System.assertEquals('bc', ascii.mid(1, 2));
System.assertEquals('Ωb', bmp.mid(1, 2));
`)
	})
	t.Run("midSupplementaryPrefix", func(t *testing.T) {
		run(t, `
String text = 'a😀b';
System.assertEquals(4, text.length());
System.assertEquals('b', text.mid(3, 1));
`)
	})

	t.Run("leftPadControls", func(t *testing.T) {
		run(t, `
String ascii = 'abc';
String bmp = 'aΩb';
System.assertEquals('  abc', ascii.leftPad(5));
System.assertEquals('  aΩb', bmp.leftPad(5));
`)
	})
	t.Run("leftPadSupplementaryWidth", func(t *testing.T) {
		run(t, `
String text = 'a😀b';
String padded = text.leftPad(6);
System.assertEquals(4, text.length());
System.assertEquals(6, padded.length());
System.assertEquals('  a😀b', padded);
`)
	})

	t.Run("rightPadControls", func(t *testing.T) {
		run(t, `
String ascii = 'abc';
String bmp = 'aΩb';
System.assertEquals('abc  ', ascii.rightPad(5));
System.assertEquals('aΩb  ', bmp.rightPad(5));
`)
	})
	t.Run("rightPadSupplementaryWidth", func(t *testing.T) {
		run(t, `
String text = 'a😀b';
String padded = text.rightPad(6);
System.assertEquals(4, text.length());
System.assertEquals(6, padded.length());
System.assertEquals('a😀b  ', padded);
`)
	})

	t.Run("centerControls", func(t *testing.T) {
		run(t, `
String ascii = 'abcd';
String bmp = 'aΩb';
System.assertEquals(' abcd ', ascii.center(6));
System.assertEquals(' aΩb ', bmp.center(5));
`)
	})
	t.Run("centerSupplementaryWidth", func(t *testing.T) {
		run(t, `
String text = 'a😀b';
String centered = text.center(6);
System.assertEquals(4, text.length());
System.assertEquals(6, centered.length());
System.assertEquals(' a😀b ', centered);
`)
	})
}

func TestStringSpanAndDefaultWidthsRejectWrongTypes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		method string
		args   []Value
	}{
		{name: "left", method: "left", args: []Value{String("1")}},
		{name: "right", method: "right", args: []Value{String("1")}},
		{name: "mid", method: "mid", args: []Value{String("0"), Int(1)}},
		{name: "leftPad", method: "leftPad", args: []Value{String("4")}},
		{name: "rightPad", method: "rightPad", args: []Value{String("4")}},
		{name: "center", method: "center", args: []Value{String("4")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, handled, err := callStringMember(String("abc"), tc.method, tc.args)
			if !handled || err == nil {
				t.Fatalf("%s should reject a String width argument: handled=%t err=%v", tc.method, handled, err)
			}
		})
	}
}
