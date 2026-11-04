package startupcache

import (
	"math/bits"

	"github.com/glade-sh/glade/internal/storage"
)

// encoding/gob flattens pointers and omits zero values, so a *bool that points
// at false (or an *int at 0) decodes as nil. storage.Field uses nil for an
// unknown describe flag and a pointer to false for a known false answer: a
// non-nillable Boolean whose Nillable flag decoded as nil was treated as
// nillable, so warm runs skipped its insert default and failed with
// REQUIRED_FIELD_MISSING. The split test payload records which flags held a
// zero value and restores them after decode.

// zeroFieldFlags lists one object's fields whose pointer flags point at zero
// values. Masks[i] belongs to Fields[i].
type zeroFieldFlags struct {
	Object string
	Fields []string
	Masks  []uint32
}

// fieldBoolFlags lists every *bool in storage.Field. Bit i of a mask is
// fieldBoolFlags[i]; zeroRelationshipOrderBit covers RelationshipOrder.
// TestZeroFieldFlagsCoverEveryPointerScalar fails when a new pointer scalar is
// added to storage.Field without an entry here.
var fieldBoolFlags = [...]func(*storage.Field) **bool{
	func(f *storage.Field) **bool { return &f.Nillable },
	func(f *storage.Field) **bool { return &f.DefaultedOnCreate },
	func(f *storage.Field) **bool { return &f.Accessible },
	func(f *storage.Field) **bool { return &f.Createable },
	func(f *storage.Field) **bool { return &f.Updateable },
	func(f *storage.Field) **bool { return &f.Filterable },
	func(f *storage.Field) **bool { return &f.Groupable },
	func(f *storage.Field) **bool { return &f.Sortable },
	func(f *storage.Field) **bool { return &f.Aggregatable },
	func(f *storage.Field) **bool { return &f.Permissionable },
	func(f *storage.Field) **bool { return &f.DeprecatedAndHidden },
	func(f *storage.Field) **bool { return &f.DescribeCaseSensitive },
}

const zeroRelationshipOrderBit = uint32(1) << len(fieldBoolFlags)

func fieldZeroMask(field *storage.Field) uint32 {
	var mask uint32
	for i, flag := range fieldBoolFlags {
		if value := *flag(field); value != nil && !*value {
			mask |= 1 << i
		}
	}
	if field.RelationshipOrder != nil && *field.RelationshipOrder == 0 {
		mask |= zeroRelationshipOrderBit
	}
	return mask
}

func captureZeroFieldFlags(org storage.OrgState) []zeroFieldFlags {
	var flags []zeroFieldFlags
	for objectName, object := range org.Objects {
		entry := zeroFieldFlags{Object: objectName}
		for fieldName, field := range object.Definition.Fields {
			if mask := fieldZeroMask(&field); mask != 0 {
				entry.Fields = append(entry.Fields, fieldName)
				entry.Masks = append(entry.Masks, mask)
			}
		}
		if len(entry.Fields) != 0 {
			flags = append(flags, entry)
		}
	}
	return flags
}

func restoreZeroFieldFlags(org *storage.OrgState, flags []zeroFieldFlags) {
	if org == nil || len(flags) == 0 {
		return
	}
	boolCount, intCount := 0, 0
	for _, entry := range flags {
		for _, mask := range entry.Masks {
			boolCount += bits.OnesCount32(mask &^ zeroRelationshipOrderBit)
			if mask&zeroRelationshipOrderBit != 0 {
				intCount++
			}
		}
	}
	// One backing array per kind; each restored flag still has its own slot.
	falses := make([]bool, boolCount)
	zeros := make([]int, intCount)
	for _, entry := range flags {
		object, ok := org.Objects[entry.Object]
		if !ok || len(entry.Fields) != len(entry.Masks) {
			continue
		}
		fields := object.Definition.Fields
		for i, name := range entry.Fields {
			field, ok := fields[name]
			if !ok {
				continue
			}
			mask := entry.Masks[i]
			for bit, flag := range fieldBoolFlags {
				if mask&(1<<bit) != 0 {
					*flag(&field) = &falses[0]
					falses = falses[1:]
				}
			}
			if mask&zeroRelationshipOrderBit != 0 {
				field.RelationshipOrder = &zeros[0]
				zeros = zeros[1:]
			}
			fields[name] = field
		}
	}
}
