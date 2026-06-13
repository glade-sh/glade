package vm

import "testing"

func TestApexUTF16IndexHelpers(t *testing.T) {
	text := "a😀b"
	if got := apexStringLength(text); got != 4 {
		t.Fatalf("apexStringLength(%q) = %d, want 4", text, got)
	}
	start, err := byteIndexForApexStringIndex(text, 1)
	if err != nil || text[start:] != "😀b" {
		t.Fatalf("byteIndexForApexStringIndex start = %d err %v", start, err)
	}
	end, err := byteIndexForApexStringIndex(text, 3)
	if err != nil || text[end:] != "b" {
		t.Fatalf("byteIndexForApexStringIndex end = %d err %v", end, err)
	}
	if _, err := byteIndexForApexStringIndex(text, 2); err == nil {
		t.Fatal("byteIndexForApexStringIndex accepted a split surrogate position")
	}
	if got, err := apexStringIndexForByteIndex(text, end); err != nil || got != 3 {
		t.Fatalf("apexStringIndexForByteIndex = %d err %v, want 3", got, err)
	}
}

func TestApexStringFromCharArrayUsesUTF16Units(t *testing.T) {
	got, err := apexStringFromCharArray(List(Int(55357), Int(56832)).List)
	if err != nil {
		t.Fatal(err)
	}
	if got != "😀" || apexStringLength(got) != 2 {
		t.Fatalf("surrogate pair produced %q length %d, want emoji length 2", got, apexStringLength(got))
	}
	truncated, err := apexStringFromCharArray(List(Int(128512)).List)
	if err != nil {
		t.Fatal(err)
	}
	if apexStringLength(truncated) != 1 {
		t.Fatalf("scalar input length = %d, want one UTF-16 unit", apexStringLength(truncated))
	}
}

func TestApexUTF16CodePointHelpers(t *testing.T) {
	text := "a😀b"
	if got, err := codePointAtApexIndex(text, 1); err != nil || got != 128512 {
		t.Fatalf("codePointAtApexIndex high surrogate = %d err %v, want 128512", got, err)
	}
	if got, err := codePointAtApexIndex(text, 2); err != nil || got != 56832 {
		t.Fatalf("codePointAtApexIndex low surrogate = %d err %v, want 56832", got, err)
	}
	if got, err := codePointBeforeApexIndex(text, 3); err != nil || got != 128512 {
		t.Fatalf("codePointBeforeApexIndex after pair = %d err %v, want 128512", got, err)
	}
	if got, err := codePointCountForApexRange(text, 0, 4); err != nil || got != 3 {
		t.Fatalf("codePointCountForApexRange = %d err %v, want 3", got, err)
	}
	if got, err := offsetApexIndexByCodePoints(text, 1, 1); err != nil || got != 3 {
		t.Fatalf("offsetApexIndexByCodePoints forward = %d err %v, want 3", got, err)
	}
	if got, err := offsetApexIndexByCodePoints(text, 3, -1); err != nil || got != 1 {
		t.Fatalf("offsetApexIndexByCodePoints backward = %d err %v, want 1", got, err)
	}
}
