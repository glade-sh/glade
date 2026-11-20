package vm

import (
	"bytes"
	"encoding/json"
	"maps"
	"strings"
	"testing"
)

func TestJSONGeneratorOutputIdentity(t *testing.T) {
	for _, pretty := range []bool{false, true} {
		t.Run(map[bool]string{false: "compact", true: "pretty"}[pretty], func(t *testing.T) {
			g := newJSONGenerator(pretty)
			_, g, _, _, _ = jsonGeneratorWriteContainer(g, "array")
			_, g, _, _, _ = jsonGeneratorWriteScalar(g, String("a\n\"β"))
			snapshot := cloneValue(g)
			before, err := json.Marshal(snapshot)
			if err != nil {
				t.Fatal(err)
			}
			_, g, _, _, _ = jsonGeneratorWriteScalar(g, Int(42))
			after, err := json.Marshal(snapshot)
			if err != nil {
				t.Fatal(err)
			}
			if string(before) != string(after) {
				t.Fatalf("snapshot changed: %s != %s", before, after)
			}
			_, g, _, _, err = jsonGeneratorEndContainer(g, "object")
			if err == nil || err.Error() != "Current context not an object but ARRAY" {
				t.Fatalf("diagnostic: %v", err)
			}
			g, err = jsonGeneratorFinish(g)
			if err != nil {
				t.Fatal(err)
			}
			want := `["a\n\"β",42]`
			if pretty {
				want = `[ "a\n\"β", 42 ]`
			}
			if got := jsonGeneratorStringField(g, "out").Text; got != want {
				t.Fatalf("output: %q; want %q", got, want)
			}
		})
	}
}

func BenchmarkJSONGeneratorWrites(b *testing.B) {
	text := String(strings.Repeat("x", 64))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		g := newJSONGenerator(false)
		_, g, _, _, _ = jsonGeneratorWriteContainer(g, "array")
		for j := 0; j < 2048; j++ {
			_, g, _, _, _ = jsonGeneratorWriteScalar(g, text)
		}
		g, _ = jsonGeneratorFinish(g)
		if got := len(jsonGeneratorStringField(g, "out").Text); got != 2048*67+1 {
			b.Fatal(got)
		}
	}
}

func TestJSONGeneratorObservationIdentity(t *testing.T) {
	for _, pretty := range []bool{false, true} {
		t.Run(map[bool]string{false: "compact", true: "pretty"}[pretty], func(t *testing.T) {
			machine := New(nil)
			g := newJSONGenerator(pretty)
			_, g, _, _, _ = jsonGeneratorWriteContainer(g, "array")
			_, g, _, _, _ = jsonGeneratorWriteScalar(g, String("first <β>"))
			snapshot := cloneValue(g)
			held := cloneValue(snapshot)
			before, err := json.Marshal(snapshot)
			if err != nil {
				t.Fatal(err)
			}
			outBefore := jsonGeneratorStringField(snapshot, "out").Text
			debugBefore := snapshot.String()
			serialized, err := json.Marshal(machine.jsonFromValueForJSONSerialize(snapshot, false))
			if err != nil {
				t.Fatal(err)
			}
			heapBefore := machine.heapTreeSize(snapshot)
			_, g, _, _, _ = jsonGeneratorWriteScalar(g, String("original"))
			_, snapshot, _, _, _ = jsonGeneratorWriteScalar(snapshot, String("copy"))
			if outBefore != map[bool]string{false: `["first <β>"`, true: `[ "first <β>"`}[pretty] {
				t.Fatalf("prior output changed: %q", outBefore)
			}
			heldBytes, err := json.Marshal(held)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, heldBytes) || held.String() != debugBefore || machine.heapTreeSize(held) != heapBefore {
				t.Fatal("held snapshot changed")
			}
			copied, err := json.Marshal(snapshot)
			if err != nil {
				t.Fatal(err)
			}
			original, err := json.Marshal(g)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("before=%s text=%q serialized=%s heap=%d original=%s copied=%s", before, debugBefore, serialized, heapBefore, original, copied)
			got, closed, _, _, err := machine.callJSONGeneratorMember(g, "getAsString", nil)
			if err != nil {
				t.Fatal(err)
			}
			_, _, _, _, closeErr := machine.callJSONGeneratorMember(closed, "writeString", []Value{String("later")})
			if closeErr == nil {
				t.Fatal("write after getAsString succeeded")
			}
			_, snapshot, _, _, err = machine.callJSONGeneratorMember(snapshot, "close", nil)
			if err != nil {
				t.Fatal(err)
			}
			copyOut := jsonGeneratorStringField(snapshot, "out").Text
			t.Logf("result=%q copy=%q closed=%t err=%q heap=%d debug=%q", got.Text, copyOut, jsonGeneratorBoolField(closed, "closed").Bool, closeErr.Error(), machine.heapTreeSize(closed), closed.String())
			wantOutput, wantCopy, wantHeapBefore, wantHeapAfter := `["first <β>","original"]`, `["first <β>","copy"]`, 244, 149
			if pretty {
				wantOutput, wantCopy, wantHeapBefore, wantHeapAfter = `[ "first <β>", "original" ]`, `[ "first <β>", "copy" ]`, 245, 152
			}
			if got.Text != wantOutput || copyOut != wantCopy || heapBefore != wantHeapBefore || machine.heapTreeSize(closed) != wantHeapAfter || closeErr.Error() != "System.UnexpectedException: Salesforce System Error" {
				t.Fatalf("outputs or diagnostics changed: %q / %q; heap %d / %d; error %v", got.Text, copyOut, heapBefore, machine.heapTreeSize(closed), closeErr)
			}
		})
	}
}

func TestJSONGeneratorAppendCopiedFields(t *testing.T) {
	original := newJSONGenerator(false)
	jsonGeneratorAppend(&original, "prefix")
	copied := original
	copied.Fields = maps.Clone(original.Fields)
	prefix := original.Fields["out"].Text
	jsonGeneratorAppend(&original, "-left")
	jsonGeneratorAppend(&copied, "-right")
	if prefix != "prefix" || original.Fields["out"].Text != "prefix-left" || copied.Fields["out"].Text != "prefix-right" {
		t.Fatalf("prefix=%q original=%q copy=%q", prefix, original.Fields["out"].Text, copied.Fields["out"].Text)
	}
}

func TestJSONGeneratorAppendOutputMetadataIdentity(t *testing.T) {
	g := newJSONGenerator(false)
	out := String("prefix")
	out.Type = "example"
	out.Int = 7
	g.Fields["out"] = out
	jsonGeneratorAppend(&g, "-suffix")
	got := g.Fields["out"]
	if got.Text != "prefix-suffix" || got.Type != "" || got.Int != 0 {
		t.Fatalf("output metadata: %+v", got)
	}
	out = g.Fields["out"]
	out.Text = "other--suffix"
	g.Fields["out"] = out
	jsonGeneratorAppend(&g, "!")
	if got := g.Fields["out"].Text; got != "other--suffix!" {
		t.Fatalf("replaced output: %q", got)
	}
}

func TestJSONGeneratorLargeOutputIdentity(t *testing.T) {
	chunk := strings.Repeat("x", 64)
	g := newJSONGenerator(false)
	_, g, _, _, _ = jsonGeneratorWriteContainer(g, "array")
	for i := 0; i < 2048; i++ {
		_, g, _, _, _ = jsonGeneratorWriteScalar(g, String(chunk))
	}
	g, err := jsonGeneratorFinish(g)
	if err != nil {
		t.Fatal(err)
	}
	want := "[" + strings.Repeat(`"`+chunk+`",`, 2047) + `"` + chunk + `"]`
	if got := jsonGeneratorStringField(g, "out").Text; got != want {
		t.Fatalf("large output differs: got %d bytes, want %d", len(got), len(want))
	}
}
