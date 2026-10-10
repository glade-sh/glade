package apextest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"hash"
	"sort"

	"github.com/glade-sh/glade/internal/storage"
)

type runtimePatchAmbientPayload struct {
	Org       storage.OrgState `json:"org"`
	PageNames []string         `json:"pageNames,omitempty"`
}

// runtimePatchAmbientDigest returns the hex SHA-256 of json.Marshal(payload)
// without building the whole encoding in memory. With the standard org that
// encoding is about 32 MB; streaming it one object at a time avoids the
// buffer growth, the final copy and most of the GC work.
func runtimePatchAmbientDigest(payload runtimePatchAmbientPayload) (string, bool) {
	h := sha256.New()
	if !writeRuntimePatchAmbientJSON(h, payload) {
		return "", false
	}
	return hex.EncodeToString(h.Sum(nil)), true
}

// writeRuntimePatchAmbientJSON writes exactly the bytes of
// json.Marshal(payload). It encodes the payload once with a nil Objects map
// and once with an empty one; the two encodings differ only at the objects
// value ("null" against "{}"), which locates it without depending on the
// OrgState layout. The map is then written as encoding/json writes it: keys
// in byte order, each key and value encoded with HTML escaping.
func writeRuntimePatchAmbientJSON(h hash.Hash, payload runtimePatchAmbientPayload) bool {
	objects := payload.Org.Objects
	if objects == nil {
		return writeRuntimePatchAmbientMarshal(h, payload)
	}
	payload.Org.Objects = nil
	withNull, err := json.Marshal(payload)
	if err != nil {
		return false
	}
	payload.Org.Objects = map[string]storage.ObjectState{}
	withEmpty, err := json.Marshal(payload)
	if err != nil {
		return false
	}
	split := 0
	for split < len(withNull) && split < len(withEmpty) && withNull[split] == withEmpty[split] {
		split++
	}
	if !bytes.HasPrefix(withNull[split:], []byte("null")) || !bytes.HasPrefix(withEmpty[split:], []byte("{}")) ||
		!bytes.Equal(withNull[split+len("null"):], withEmpty[split+len("{}"):]) {
		payload.Org.Objects = objects
		return writeRuntimePatchAmbientMarshal(h, payload)
	}
	names := make([]string, 0, len(objects))
	for name := range objects {
		names = append(names, name)
	}
	sort.Strings(names)
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encode := func(value any) bool {
		buf.Reset()
		if encoder.Encode(value) != nil {
			return false
		}
		// Encode appends a newline that Marshal does not.
		_, _ = h.Write(bytes.TrimSuffix(buf.Bytes(), []byte("\n")))
		return true
	}
	_, _ = h.Write(withNull[:split])
	_, _ = h.Write([]byte("{"))
	for i, name := range names {
		if i > 0 {
			_, _ = h.Write([]byte(","))
		}
		if !encode(name) {
			return false
		}
		_, _ = h.Write([]byte(":"))
		if !encode(objects[name]) {
			return false
		}
	}
	_, _ = h.Write([]byte("}"))
	_, _ = h.Write(withNull[split+len("null"):])
	return true
}

func writeRuntimePatchAmbientMarshal(h hash.Hash, payload runtimePatchAmbientPayload) bool {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return false
	}
	_, _ = h.Write(encoded)
	return true
}
