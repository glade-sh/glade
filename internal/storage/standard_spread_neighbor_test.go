package storage

import (
	"crypto/sha256"
	"encoding/json"
	"reflect"
	"testing"
)

// The digest is the serialized result on origin/main at 0574a2848cf9, plus the
// native relationship orders of relationship_order_standard/org.tsv and the
// captured MessagingSession.CampaignId supplement
// (messaging_campaign_presence/facets.tsv).
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
	if got := sha256.Sum256(data); got != [32]byte{0x28, 0x24, 0x05, 0x20, 0xb4, 0xce, 0x4b, 0x4c, 0x45, 0x00, 0x52, 0x1a, 0x2c, 0x78, 0xd7, 0x18, 0xea, 0xae, 0x72, 0x67, 0x82, 0x25, 0x62, 0xe6, 0x9c, 0xd8, 0x21, 0x93, 0x09, 0xb4, 0x19, 0x7c} {
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
