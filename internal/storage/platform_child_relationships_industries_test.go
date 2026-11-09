package storage

import (
	"os"
	"strings"
	"testing"
)

type industriesChildRow struct {
	api, org, parent, name, child, field string
	present                              bool
}

func readIndustriesChildRows(t *testing.T) []industriesChildRow {
	t.Helper()
	data, err := os.ReadFile("../apextest/testdata/conformance/industries_standard_objects/child_relationships.tsv")
	if err != nil {
		t.Fatal(err)
	}
	var rows []industriesChildRow
	for i, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if i == 0 {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) != 7 {
			t.Fatalf("line %d: %d columns", i+1, len(f))
		}
		rows = append(rows, industriesChildRow{f[0], f[1], f[2], f[3], f[4], f[5], f[6] == "true"})
	}
	return rows
}

func industriesDefinition(parent string, features ...string) ObjectDefinition {
	return ObjectDefinition{APIName: parent, Metadata: map[string]string{standardFieldsOverlayMarker: canonicalFeatureSignature(features)}}
}

// Every shipped edge has a native describe row at API 62.0 and 67.0 in an org
// with PublicSectorAccess, and the lookup resolves it for that org shape.
func TestIndustriesChildRelationshipsMatchNativeDescribe(t *testing.T) {
	rows := readIndustriesChildRows(t)
	native := map[string]bool{}
	for _, r := range rows {
		if r.org == "PublicSectorAccess" && r.present {
			native[r.api+"|"+r.parent+"|"+r.name+"|"+r.child+"|"+r.field] = true
		}
	}
	shipped := 0
	for parent, edges := range industriesChildRelationshipSupplements {
		for _, e := range edges {
			shipped++
			for _, api := range []string{"62.0", "67.0"} {
				if !native[api+"|"+parent+"|"+e.name+"|"+e.child+"|"+e.field] {
					t.Errorf("shipped edge %s.%s (%s.%s) has no native row at %s", parent, e.name, e.child, e.field, api)
				}
			}
			child, rel, ok := LookupPlatformChildRelationship(industriesDefinition(parent, "PublicSectorAccess"), e.name)
			if !ok || child != e.child || rel.Field != e.field {
				t.Errorf("%s.%s not resolved with the feature: %q %q %v", parent, e.name, child, rel.Field, ok)
			}
		}
	}
	if shipped != 131 {
		t.Fatalf("shipped %d edges, native capture covers 131", shipped)
	}
}

// Without PublicSectorAccess the org reports only the ungated edges; the lookup
// and the child-side naming follow the capture row for row.
func TestIndustriesChildRelationshipsGatedOnPublicSectorAccess(t *testing.T) {
	gated, ungated := 0, 0
	for _, r := range readIndustriesChildRows(t) {
		if r.org != "none" {
			continue
		}
		child, rel, ok := LookupPlatformChildRelationship(industriesDefinition(r.parent), r.name)
		visible := ok && child == r.child && rel.Field == r.field
		if visible != r.present {
			t.Errorf("%s %s.%s without feature: visible=%v, native present=%v", r.api, r.parent, r.name, visible, r.present)
		}
		childDef := industriesDefinition(r.child)
		childDef.Relations = []Relationship{{Field: r.field}}
		NameIndustriesChildRelationships(&childDef)
		if named := childDef.Relations[0].ChildRelationship == r.name; named != r.present {
			t.Errorf("%s %s.%s child-side name without feature: named=%v, native present=%v", r.api, r.parent, r.name, named, r.present)
		}
		if r.present {
			ungated++
		} else {
			gated++
		}
	}
	if gated == 0 || ungated == 0 {
		t.Fatalf("capture has %d gated and %d ungated rows", gated, ungated)
	}
}

// A definition with no applied feature signature or from a project that declares
// no org shape cannot establish absence, so every edge stays visible.
func TestIndustriesChildRelationshipsVisibleWhenShapeUnknown(t *testing.T) {
	for _, parent := range []ObjectDefinition{{APIName: "Account"}, industriesDefinition("Account", "__glade_unspecified_standard_field_shape")} {
		if _, _, ok := LookupPlatformChildRelationship(parent, "BusinessProfiles"); !ok {
			t.Errorf("BusinessProfiles hidden for %v", parent.Metadata)
		}
	}
}
