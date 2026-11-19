package vm

import (
	"encoding/json"
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
