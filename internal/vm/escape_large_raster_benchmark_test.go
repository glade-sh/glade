package vm

import (
	"crypto/sha256"
	"fmt"
	"testing"
)

func BenchmarkEscapeLargeRasterWriteback(b *testing.B) {
	const pixels = 512 * 384
	values := make([]Value, pixels, pixels+1)
	for i := range values {
		values[i] = Int(int64(i % 256))
	}
	previous := List(values...)
	updated := previous
	updated.List = append(updated.List, Int(0))
	machine := New(nil)
	local := Object("Pending")
	machine.rememberLocalOnlyObject(local)
	check := func() {
		data := make([]byte, len(updated.List))
		for i, value := range updated.List {
			if value.Kind != ValueInt || value.Int < 0 || value.Int > 255 {
				b.Fatalf("pixel %d is not a byte", i)
			}
			data[i] = byte(value.Int)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != "5ff0de24327cc652cb7e05ddf94b7d8dd220e8098f08f52199cf5b231114df21" {
			b.Fatalf("pixel digest = %s", got)
		}
		if !machine.localOnlyAlias(snapshotAlias(local)) {
			b.Fatal("unrelated object escaped")
		}
	}
	check()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		machine.markCollectionRefsEscapedSince(previous, updated)
	}
	b.StopTimer()
	check()
}
