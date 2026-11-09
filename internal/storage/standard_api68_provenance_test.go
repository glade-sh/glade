package storage

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestStandardAPI68DescribeNativeSourceDigests(t *testing.T) {
	stream, err := gzip.NewReader(bytes.NewReader(standardAPI68DescribeData))
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	var bundle struct {
		Sources map[string]string `json:"sources"`
	}
	if err := json.NewDecoder(stream).Decode(&bundle); err != nil {
		t.Fatal(err)
	}
	if len(bundle.Sources) == 0 {
		t.Fatal("missing source digests")
	}
	root := "../apextest/testdata/conformance"
	for name, want := range bundle.Sources {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != want {
			t.Errorf("%s digest=%s want=%s", name, got, want)
		}
	}
}
