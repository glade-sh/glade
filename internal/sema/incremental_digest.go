package sema

import (
	"crypto/sha256"
	"encoding/binary"
	"hash"
	"reflect"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
)

var diagnosticRangeType = reflect.TypeOf(diagnostic.Range{})

// positionBlindDigest hashes every exported field reachable from value except
// source positions: a diagnostic.Range anywhere in the graph contributes
// nothing. Two indexes that differ only in where their declarations sit hash
// the same, which is what a body-only edit produces. Unexported fields are
// not hashed; they carry capture identity, which the caller hashes itself.
func positionBlindDigest(value any) [sha256.Size]byte {
	h := &blindHasher{h: sha256.New(), seen: make(map[uintptr]bool)}
	h.value(reflect.ValueOf(value))
	var out [sha256.Size]byte
	h.h.Sum(out[:0])
	return out
}

type blindHasher struct {
	h    hash.Hash
	seen map[uintptr]bool
	buf  [8]byte
}

func (b *blindHasher) tag(t byte) {
	b.buf[0] = t
	b.h.Write(b.buf[:1])
}

func (b *blindHasher) uint(v uint64) {
	binary.LittleEndian.PutUint64(b.buf[:], v)
	b.h.Write(b.buf[:])
}

func (b *blindHasher) str(s string) {
	b.uint(uint64(len(s)))
	b.h.Write([]byte(s))
}

func (b *blindHasher) value(v reflect.Value) {
	switch v.Kind() {
	case reflect.Invalid:
		b.tag('0')
	case reflect.Bool:
		b.tag('b')
		if v.Bool() {
			b.uint(1)
		} else {
			b.uint(0)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		b.tag('i')
		b.uint(uint64(v.Int()))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		b.tag('u')
		b.uint(v.Uint())
	case reflect.Float32, reflect.Float64:
		b.tag('f')
		b.uint(uint64(int64(v.Float() * 1e9)))
	case reflect.String:
		b.tag('s')
		b.str(v.String())
	case reflect.Slice:
		b.tag('[')
		b.uint(uint64(v.Len()))
		for i := 0; i < v.Len(); i++ {
			b.value(v.Index(i))
		}
	case reflect.Array:
		b.tag('[')
		b.uint(uint64(v.Len()))
		for i := 0; i < v.Len(); i++ {
			b.value(v.Index(i))
		}
	case reflect.Map:
		b.tag('{')
		b.uint(uint64(v.Len()))
		b.mapEntries(v)
	case reflect.Struct:
		if v.Type() == diagnosticRangeType {
			b.tag('r')
			return
		}
		b.tag('S')
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			if !t.Field(i).IsExported() {
				continue
			}
			b.str(t.Field(i).Name)
			b.value(v.Field(i))
		}
	case reflect.Pointer:
		if v.IsNil() {
			b.tag('n')
			return
		}
		if v.Type().Elem() == diagnosticRangeType {
			b.tag('r')
			return
		}
		ptr := v.Pointer()
		if b.seen[ptr] {
			b.tag('c')
			return
		}
		b.seen[ptr] = true
		b.tag('p')
		b.value(v.Elem())
		delete(b.seen, ptr)
	case reflect.Interface:
		if v.IsNil() {
			b.tag('n')
			return
		}
		b.tag('I')
		b.str(v.Elem().Type().String())
		b.value(v.Elem())
	default:
		// Funcs, channels and unsafe pointers carry no analyzable content.
		b.tag('x')
	}
}

func (b *blindHasher) mapEntries(v reflect.Value) {
	type entry struct {
		key  string
		hash [sha256.Size]byte
	}
	entries := make([]entry, 0, v.Len())
	iter := v.MapRange()
	for iter.Next() {
		sub := &blindHasher{h: sha256.New(), seen: b.seen}
		sub.value(iter.Key())
		sub.value(iter.Value())
		var e entry
		sub.h.Sum(e.hash[:0])
		if iter.Key().Kind() == reflect.String {
			e.key = iter.Key().String()
		}
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].key != entries[j].key {
			return entries[i].key < entries[j].key
		}
		return strings.Compare(string(entries[i].hash[:]), string(entries[j].hash[:])) < 0
	})
	for _, e := range entries {
		b.h.Write(e.hash[:])
	}
}
