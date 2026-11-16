package storage

import (
	"crypto/sha256"
	"encoding/json"
	"reflect"
	"testing"
)

// The digest is the serialized result on origin/main at 0574a2848cf9, plus the
// native relationship orders of relationship_order_standard/org.tsv.
// This uncaptured shape exercises lazy installs followed by a completed spread.
func TestSpreadReleaseNeighborDigest(t *testing.T) {
	was := SetStandardCatalogReleaseEnabled(true)
	defer SetStandardCatalogReleaseEnabled(was)
	org := sourceSpreadLazyOrg([]string{"MultiCurrency"})
	for _, name := range []string{"Account", "Contact", "Opportunity"} {
		EnsureStandardObject(&org, name)
	}
	MaterializeStandardObjectSpread(&org)
	data, err := json.Marshal(org)
	if err != nil {
		t.Fatal(err)
	}
	if got := sha256.Sum256(data); got != [32]byte{0xd0, 0x9e, 0x38, 0xa8, 0x46, 0x6d, 0xf6, 0xda, 0x11, 0x63, 0x9d, 0x66, 0x60, 0x66, 0x98, 0xb8, 0x0c, 0x68, 0x13, 0x04, 0x79, 0xb6, 0xbd, 0xb7, 0xb9, 0x4e, 0x42, 0x46, 0x40, 0x4f, 0xc8, 0x13} {
		t.Fatalf("serialized neighbor differs from origin/main: %x", got)
	}
}

func TestSpreadTemplateOrgIdentityForSafeCells(t *testing.T) {
	for _, phase := range []string{standardSpreadPlain, standardSpreadShaped} {
		checked := 0
		for _, name := range KnownStandardObjectNames() {
			first := NewOrgState()
			first.OrgID = "first"
			first.EnableStandardSpreadSource()
			first.StandardSpread = phase
			second := NewOrgState()
			second.OrgID = "second"
			second.EnableStandardSpreadSource()
			second.StandardSpread = phase
			if _, owed := spreadObjectOwed(&first, name); !owed {
				continue
			}
			spreadTemplates.Clear()
			a, aok := spreadTemplate(&first, name)
			spreadTemplates.Clear()
			b, bok := spreadTemplate(&second, name)
			if aok != bok || !reflect.DeepEqual(a, b) {
				t.Fatalf("%s %s differs across identities", phase, name)
			}
			if aok {
				checked++
			}
			if checked == 2 {
				break
			}
		}
		spreadTemplates.Clear()
		if checked != 2 {
			t.Fatalf("%s: only %d safe cells", phase, checked)
		}
	}
}

// A never-dropped in-flight cell keeps the original nonblocking answer. The
// reset matters: an earlier test's release may have dropped this plain key,
// and a dropped key waits for its rebuild instead.
func TestSpreadObjectUntouchedPendingNeighbor(t *testing.T) {
	resetSpreadMemos(t)
	plain := NewOrgState()
	plain.OrgID = DefaultOrgID
	plain.EnableStandardSpreadSource()
	name := ""
	for _, candidate := range KnownStandardObjectNames() {
		if _, owed := spreadObjectOwed(&plain, candidate); owed {
			name = candidate
			break
		}
	}
	if name == "" {
		t.Fatal("no owed object")
	}
	if _, ok := installSpreadObject(&plain, name); !ok {
		t.Fatal("plain install failed")
	}
	_, hasRecordType := plain.Objects["RecordType"]
	key := spreadTemplateKey{phase: plain.StandardSpread, signature: canonicalFeatureSignature(plain.Features), multiCurr: plain.HasFeature("MultiCurrency"), recordType: hasRecordType, object: name}
	old, loaded := spreadTemplates.Load(key)
	if !loaded {
		t.Fatal("installed template missing")
	}
	spreadTemplates.Store(key, &spreadTemplateCell{})
	defer spreadTemplates.Store(key, old)
	if plain.SpreadObjectUntouched(name) {
		t.Fatal("pending template reported untouched")
	}
}
