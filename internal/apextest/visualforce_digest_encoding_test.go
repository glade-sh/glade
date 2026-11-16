package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestVisualforceDigestEntryUsesCompactEncoding(t *testing.T) {
	want := sha256.Sum256([]byte("content"))
	legacy, err := json.Marshal(struct{ Digest [sha256.Size]byte }{want})
	if err != nil {
		t.Fatal(err)
	}
	var entry visualforceDigestEntry
	if err := json.Unmarshal(legacy, &entry); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	if digest := fields["Digest"]; len(digest) == 0 || digest[0] != '"' {
		t.Fatalf("digest is not encoded as a compact string: %s", digest)
	}
	var decoded visualforceDigestEntry
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Digest) != len(want) {
		t.Fatalf("decoded digest length %d", len(decoded.Digest))
	}
	for i, b := range decoded.Digest {
		if b != want[i] {
			t.Fatalf("digest differs at byte %d", i)
		}
	}
}

func TestVisualforceDigestEntryRejectsShortDigest(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "input")
	if err := os.WriteFile(path, []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := saveVisualforceDigestCache(root, []string{path}); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal([]struct {
		Path   string
		Digest json.RawMessage
	}{{Path: path, Digest: json.RawMessage(`[1,2]`)}})
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(visualforceDigestEnvelope{Version: 1, Payload: payload, SHA256: sha256.Sum256(payload)})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(visualforceDigestCachePath(root), data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := loadVisualforceDigestCache(root, []string{path}); err == nil {
		t.Fatal("short digest accepted")
	}
}
