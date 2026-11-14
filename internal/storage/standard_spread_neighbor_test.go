package storage

import (
	"crypto/sha256"
	"encoding/json"
	"reflect"
	"testing"
)

// The digest is the serialized result on origin/main at 0574a2848cf9.
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
	if got := sha256.Sum256(data); got != [32]byte{0x03, 0xfa, 0x44, 0xb0, 0x08, 0xe2, 0xa2, 0x11, 0x2c, 0x1d, 0x58, 0x39, 0x22, 0x95, 0xb2, 0xaa, 0x1f, 0xba, 0x1a, 0x38, 0x40, 0xc9, 0x83, 0x46, 0xcb, 0x56, 0x28, 0x3c, 0xe7, 0x61, 0xe6, 0xd4} {
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
