package vm

import (
	"archive/zip"
	binaryencoding "encoding/binary"
	"fmt"
	"strings"
	"testing"
)

// Check the public ZIP format, not Salesforce's behavior for oversized archives.
// A local unsupported-format error is preferable to emitting a wrapped count.
func TestCompressionZipArchiveEntryCountDoesNotWrap(t *testing.T) {
	for _, count := range []int{2, 1<<16 - 1, 1 << 16} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			entries := make([]Value, count)
			for i := range entries {
				entry := Object("compression.ZipEntry")
				entry.Fields["name"] = String(fmt.Sprintf("entry-%05d", i))
				entry.Fields["method"] = compressionEnumValue("compression.Method", "STORED")
				entries[i] = entry
			}
			archive, err := writeCompressionZipArchive(entries, Null)
			if err != nil {
				if count < 1<<16 {
					t.Fatal(err)
				}
				if runtimeErr, ok := err.(*RuntimeError); !ok || runtimeErr.Type != "UnsupportedFeature" {
					t.Fatalf("oversized archive returned %v, want a local unsupported-format error", err)
				}
				if archive != "" {
					t.Fatal("unsupported archive returned partial bytes")
				}
				return
			}
			if len(archive) < 22 {
				t.Fatalf("archive has only %d bytes", len(archive))
			}
			end := []byte(archive[len(archive)-22:])
			if got := binaryencoding.LittleEndian.Uint32(end); got != 0x06054b50 {
				t.Fatalf("missing end-of-central-directory record: %x", got)
			}
			for _, offset := range []int{8, 10} {
				if got := int(binaryencoding.LittleEndian.Uint16(end[offset:])); got != count {
					t.Fatalf("end-of-central-directory count = %d, want %d", got, count)
				}
			}
			reader, err := zip.NewReader(strings.NewReader(archive), int64(len(archive)))
			if err != nil {
				t.Fatal(err)
			}
			if len(reader.File) != count {
				t.Fatalf("ZIP reader found %d entries, want %d", len(reader.File), count)
			}
		})
	}
}

func TestCompressionZipUint32Bounds(t *testing.T) {
	for _, tc := range []struct {
		value       int64
		want        uint32
		unsupported bool
	}{
		{value: -1, unsupported: true},
		{value: 0, want: 0},
		{value: 1<<32 - 2, want: 0xfffffffe},
		{value: 1<<32 - 1, unsupported: true},
		{value: 1 << 32, unsupported: true},
	} {
		t.Run(fmt.Sprint(tc.value), func(t *testing.T) {
			got, err := compressionZipUint32(tc.value)
			if tc.unsupported {
				if runtimeErr, ok := err.(*RuntimeError); !ok || runtimeErr.Type != "UnsupportedFeature" {
					t.Fatalf("out-of-range value returned %v, want a local unsupported-format error", err)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("conversion = %d, %v; want %d, nil", got, err, tc.want)
			}
		})
	}
}
