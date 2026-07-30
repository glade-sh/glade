package startupcache

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func representativeTestCacheHeader(root string, fileCount int) testCacheHeader {
	files := make([]File, 0, fileCount)
	for i := 0; i < fileCount; i++ {
		files = append(files, File{
			Path:    fmt.Sprintf("force-app/main/default/classes/GenericClass%04d.cls", i),
			Size:    int64(80 + i),
			ModTime: int64(1_800_000_000_000_000_000 + i),
			SHA256:  strings.Repeat(fmt.Sprintf("%x", i%16), sha256HexLength),
		})
	}
	hash := strings.Repeat("a", sha256HexLength)
	return testCacheHeader{
		FormatVersion: testCacheFormatVersion,
		Version:       Version,
		ProjectRoot:   root,
		BuiltAt:       "2026-07-29T12:00:00Z",
		PlatformABI:   "darwin-arm64-go1.24",
		RuntimeABI:    "runtime-v1",
		RuntimeKey:    "runtime-key",
		Manifest: Manifest{
			SchemaVersion:    manifestSchemaVersion,
			ProjectRoot:      root,
			SourceAPIVersion: "65.0",
			Namespace:        "generic",
			ProjectDigest:    strings.Repeat("b", sha256HexLength),
			Features:         []string{"Communities", "PersonAccounts"},
			Complete:         true,
			Files:            files,
			ConfigFiles: []File{{
				Path:    "sfdx-project.json",
				Size:    42,
				ModTime: 1_800_000_000_000_000_100,
				SHA256:  strings.Repeat("c", sha256HexLength),
			}},
			PackageRoots: []Directory{{
				Path:    "force-app",
				ModTime: 1_800_000_000_000_000_200,
			}},
		},
		PayloadFile:   payloadFilePrefix + hash + payloadFileSuffix,
		PayloadSHA256: hash,
		PayloadSize:   123456,
	}
}

func TestTestCacheHeaderBinaryRoundTripPreservesInvalidationInputs(t *testing.T) {
	want := representativeTestCacheHeader(filepath.Clean("/tmp/generic-project"), 8)

	data, err := marshalTestCacheHeader(want)
	if err != nil {
		t.Fatalf("marshalTestCacheHeader() error = %v", err)
	}
	got, err := unmarshalTestCacheHeader(data)
	if err != nil {
		t.Fatalf("unmarshalTestCacheHeader() error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip mismatch:\n got=%#v\nwant=%#v", got, want)
	}
}

func TestTestCacheHeaderBinaryEncodingIsDeterministicAndCompact(t *testing.T) {
	header := representativeTestCacheHeader(filepath.Clean("/tmp/generic-project"), 500)

	first, err := marshalTestCacheHeader(header)
	if err != nil {
		t.Fatal(err)
	}
	second, err := marshalTestCacheHeader(header)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("identical headers produced different bytes")
	}
	jsonData, err := json.MarshalIndent(header, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if len(first)*2 > len(jsonData) {
		t.Fatalf("binary header = %d bytes, JSON header = %d bytes; want at least 50%% smaller", len(first), len(jsonData))
	}
	t.Logf("binary header = %d bytes, JSON header = %d bytes, reduction = %.2f%%", len(first), len(jsonData), 100*(1-float64(len(first))/float64(len(jsonData))))
}

func TestTestCacheHeaderBinaryRejectsTruncationAndCorruption(t *testing.T) {
	data, err := marshalTestCacheHeader(representativeTestCacheHeader(filepath.Clean("/tmp/generic-project"), 4))
	if err != nil {
		t.Fatal(err)
	}
	for length := 0; length < len(data); length++ {
		if _, err := unmarshalTestCacheHeader(data[:length]); err == nil {
			t.Fatalf("unmarshalTestCacheHeader() accepted truncation at %d of %d bytes", length, len(data))
		}
	}
	for _, offset := range []int{0, len(testCacheHeaderMagic), len(data) / 3, len(data) / 2, len(data) - 1} {
		corrupt := append([]byte(nil), data...)
		corrupt[offset] ^= 0xff
		if _, err := unmarshalTestCacheHeader(corrupt); err == nil {
			t.Fatalf("unmarshalTestCacheHeader() accepted corruption at offset %d", offset)
		}
	}
}

func TestTestCacheHeaderBinaryRejectsOversizedPathsAndLengths(t *testing.T) {
	header := representativeTestCacheHeader(filepath.Clean("/tmp/generic-project"), 1)
	header.Manifest.Files[0].Path = strings.Repeat("x", maxTestCacheHeaderPathBytes+1)
	if _, err := marshalTestCacheHeader(header); err == nil {
		t.Fatal("marshalTestCacheHeader() accepted oversized manifest path")
	}

	header = representativeTestCacheHeader(filepath.Clean("/tmp/generic-project"), 1)
	data, err := marshalTestCacheHeader(header)
	if err != nil {
		t.Fatal(err)
	}
	oversized := append(append([]byte(nil), data...), 0)
	if _, err := unmarshalTestCacheHeader(oversized); err == nil {
		t.Fatal("unmarshalTestCacheHeader() accepted trailing bytes")
	}
	if _, err := unmarshalTestCacheHeader(make([]byte, maxTestCacheHeaderBytes+1)); err == nil {
		t.Fatal("unmarshalTestCacheHeader() accepted oversized input")
	}

	path := filepath.Join(t.TempDir(), stateHeaderFile)
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(maxEncodedTestCacheHeaderBytes + 1); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := readTestCacheHeaderAtPath(t, path); !errors.Is(err, errMalformedTestCacheHeader) {
		t.Fatalf("readTestCacheHeader() oversized error = %v, want %v", err, errMalformedTestCacheHeader)
	}
}

func TestSplitCacheHeaderAndPayloadWritesArePrivate(t *testing.T) {
	if os.PathSeparator == '\\' {
		t.Skip("Unix permission bits are not available on Windows")
	}
	root := t.TempDir()
	entry := currentStartupCacheTestEntry(t, root)
	if err := Write(&entry, SubdirTest); err != nil {
		t.Fatal(err)
	}
	headerPath := filepath.Join(root, ".glade", "test", stateHeaderFile)
	header, err := readTestCacheHeaderAtPath(t, headerPath)
	if err != nil {
		t.Fatal(err)
	}
	if header.FormatVersion != testCacheFormatVersion {
		t.Fatalf("header format version = %d, want %d", header.FormatVersion, testCacheFormatVersion)
	}
	if !validPayloadFileName(header.PayloadFile, header.PayloadSHA256) {
		t.Fatalf("invalid payload identity after round trip: %#v", header)
	}
	if !freshManifest(header.Version, header.ProjectRoot, header.PlatformABI, header.Manifest, root, Version) {
		t.Fatalf("written header is not fresh after round trip: %#v", header)
	}
	for _, path := range []string{headerPath, filepath.Join(root, ".glade", "test", header.PayloadFile)} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := info.Mode().Perm(), os.FileMode(0o600); got != want {
			t.Fatalf("%s permissions = %04o, want %04o", filepath.Base(path), got, want)
		}
	}
}
