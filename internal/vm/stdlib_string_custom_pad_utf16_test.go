package vm

import (
	"fmt"
	"testing"
)

func TestExecStringCustomPadWidthsUseUTF16API67(t *testing.T) {
	for _, tc := range []struct {
		name          string
		receiver      string
		receiverUnits int
		width         int
		method        string
		expected      string
	}{
		{name: "leftPadASCII", receiver: "abc", receiverUnits: 3, width: 5, method: "leftPad", expected: "--abc"},
		{name: "leftPadBMP", receiver: "aΩb", receiverUnits: 3, width: 5, method: "leftPad", expected: "--aΩb"},
		{name: "leftPadSupplementary", receiver: "a😀b", receiverUnits: 4, width: 6, method: "leftPad", expected: "--a😀b"},
		{name: "rightPadASCII", receiver: "abc", receiverUnits: 3, width: 5, method: "rightPad", expected: "abc--"},
		{name: "rightPadBMP", receiver: "aΩb", receiverUnits: 3, width: 5, method: "rightPad", expected: "aΩb--"},
		{name: "rightPadSupplementary", receiver: "a😀b", receiverUnits: 4, width: 6, method: "rightPad", expected: "a😀b--"},
		{name: "centerASCII", receiver: "abc", receiverUnits: 3, width: 5, method: "center", expected: "-abc-"},
		{name: "centerBMP", receiver: "aΩb", receiverUnits: 3, width: 5, method: "center", expected: "-aΩb-"},
		{name: "centerSupplementary", receiver: "a😀b", receiverUnits: 4, width: 6, method: "center", expected: "-a😀b-"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := fmt.Sprintf(`
String text = '%s';
System.assertEquals(%d, text.length());
String padded = text.%s(%d, '-');
System.assertEquals('%s', padded);
System.assertEquals(%d, padded.length());
`, tc.receiver, tc.receiverUnits, tc.method, tc.width, tc.expected, tc.width)
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
		})
	}
}
