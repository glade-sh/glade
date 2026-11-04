package storage

import (
	"sort"
	"strings"
)

// The scalar describe facets (keyPrefix and triggerable) are read for many
// standard objects before, or without, their field graphs. Decoding a packed
// member for two scalars costs a full gunzip plus decode and would have to run
// again for the field graph, so the scalars live in a small generated table
// aligned with the V2 pack. TestStandardDescribeCatalogV2ScalarsMatchPack
// checks every member against the table and regenerates it on request.
//
//go:generate sh -c "GLADE_UPDATE_DESCRIBE_SCALARS=1 go test . -run TestStandardDescribeCatalogV2ScalarsMatchPack"
type standardDescribeCatalogV2Scalar struct {
	Name      string
	KeyPrefix string
	Facets    standardDescribeScalarFacets
}

// standardDescribeScalarFacets records which scalar facets a member states.
// A describe response may carry null for either facet.
type standardDescribeScalarFacets uint8

const (
	standardDescribeScalarHasKeyPrefix standardDescribeScalarFacets = 1 << iota
	standardDescribeScalarHasTriggerable
	standardDescribeScalarTriggerable
)

func standardDescribeScalarFacetsOf(describe standardDescribeObject) standardDescribeScalarFacets {
	var facets standardDescribeScalarFacets
	if describe.KeyPrefix != nil {
		facets |= standardDescribeScalarHasKeyPrefix
	}
	if describe.Triggerable != nil {
		facets |= standardDescribeScalarHasTriggerable
		if *describe.Triggerable {
			facets |= standardDescribeScalarTriggerable
		}
	}
	return facets
}

// describe returns the scalar facets as a describe value with only KeyPrefix
// and Triggerable populated, the shape scalarForName hands out.
func (scalar standardDescribeCatalogV2Scalar) describe() standardDescribeObject {
	var describe standardDescribeObject
	if scalar.Facets&standardDescribeScalarHasKeyPrefix != 0 {
		prefix := scalar.KeyPrefix
		describe.KeyPrefix = &prefix
	}
	if scalar.Facets&standardDescribeScalarHasTriggerable != 0 {
		triggerable := scalar.Facets&standardDescribeScalarTriggerable != 0
		describe.Triggerable = &triggerable
	}
	return describe
}

// lookupStandardDescribeCatalogV2Scalar resolves one resolved index entry to
// its generated scalar facets. The table keeps the index order, so the same
// case-folded binary search applies. A miss means the table is stale for this
// member and the caller decodes the member instead.
func lookupStandardDescribeCatalogV2Scalar(entry standardDescribeCatalogV2IndexEntry) (standardDescribeObject, bool) {
	table := standardDescribeCatalogV2Scalars[:]
	key := strings.ToLower(entry.Name)
	position := sort.Search(len(table), func(position int) bool {
		return standardDescribeNameAtLeast(table[position].Name, key)
	})
	if position == len(table) || table[position].Name != entry.Name {
		return standardDescribeObject{}, false
	}
	return table[position].describe(), true
}
