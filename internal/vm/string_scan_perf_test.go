package vm

import (
	"strings"
	"testing"
)

func TestStringScanBoundsUTF16Allocations(t *testing.T) {
	for _, size := range []int{2048, 65536} {
		text := String(strings.Repeat("abc|", size/4))
		machine := New(nil)
		args := []Value{Int(int64(size - 1))}
		allocations := testing.AllocsPerRun(20, func() {
			value, handled, err := machine.callValueMember("", text, "charAt", args, nil)
			if err != nil || !handled || value.Int != 124 {
				t.Fatalf("charAt = %v, handled=%v, err=%v", value, handled, err)
			}
		})
		if allocations > 1 {
			t.Errorf("%d-byte repeated charAt: %.0f allocations per call; want <= 1 without rebuilding UTF-16", size, allocations)
		}
	}
}

func TestStringASCIICacheBoundsAndIsolation(t *testing.T) {
	machine := New(nil)
	for i := 0; i < 20; i++ {
		text := strings.Repeat(string(rune('A'+i)), 1<<20)
		if !machine.stringIsASCII(text) || machine.stringASCIIBytes > stringASCIIMaxBytes || len(machine.stringASCII) > stringASCIIMaxEntries {
			t.Fatal("ASCII classification cache exceeded its bounds")
		}
	}
	for i := 0; i < 20; i++ {
		machine.stringIsASCII(strings.Repeat(string(rune('a'+i)), 256))
		if len(machine.stringASCII) > stringASCIIMaxEntries {
			t.Fatal("ASCII classification cache exceeded its entry bound")
		}
	}
	if machine.stringIsASCII(strings.Repeat("x", 256) + "Ω") {
		t.Fatal("non-ASCII string classified as ASCII")
	}
	machine.stringIsASCII(strings.Repeat("z", stringASCIIMaxBytes+1))
	if machine.stringASCIIBytes > stringASCIIMaxBytes {
		t.Fatal("oversized string retained")
	}
	clone := machine.cloneRuntime(nil, false)
	if len(clone.stringASCII) != 0 || clone.stringASCIIBytes != 0 {
		t.Fatal("runtime clone retained another VM's string cache")
	}
}

func TestCachedStringScanPreservesResults(t *testing.T) {
	machine := New(nil)
	for _, text := range []string{"", "abc", strings.Repeat("abc|", 128), strings.Repeat("abc|", 128) + "Ω😀"} {
		for _, method := range []string{"length", "charAt", "indexOf", "substring"} {
			for _, args := range [][]Value{
				nil, {Int(-1)}, {Int(0)}, {Int(1)}, {Int(512)}, {Int(513)},
				{Int(0), Int(1)}, {Int(2), Int(1)}, {Int(0), Int(513)},
				{String("abc")}, {String("|")}, {String("Ω"), Int(1)},
				{String(""), Int(-1)}, {String(""), Int(513)},
				{String("|"), Int(2)}, {Null}, {String("a"), String("bad")},
			} {
				want, wantHandled, wantErr := callStringMember(String(text), method, args)
				got, handled, err := machine.callValueMember("", String(text), method, args, nil)
				if handled != wantHandled || got.Kind != want.Kind || got.Text != want.Text || got.Int != want.Int || errorText(err) != errorText(wantErr) {
					t.Fatalf("%s(%v), %d bytes: got %v/%v/%v; want %v/%v/%v", method, args, len(text), got, handled, err, want, wantHandled, wantErr)
				}
			}
		}
	}
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func BenchmarkStringDelimiterScan(b *testing.B) {
	text := String(strings.Repeat("abc|", 4096) + "#end")
	machine := New(nil)
	for b.Loop() {
		position := 0
		for position < len(text.Text) {
			value, handled, err := machine.callValueMember("", text, "indexOf", []Value{String("|"), Int(int64(position))}, nil)
			if err != nil || !handled {
				b.Fatal(err)
			}
			if value.Int < 0 {
				break
			}
			position = int(value.Int) + 1
		}
		if position != 16384 {
			b.Fatal(position)
		}
	}
}
