package vm

import "strings"

// Platform producers retain System identity even when a project declares Address.
// Keep the short display type used by Address accessors and locationCoordinate.
func newSystemAddress() Value {
	address := Object("Address")
	address.Runtime = "System.Address"
	return address
}

// Address's Location base is a System value type, distinct from Schema records
// and source classes. Handle only these roots; ordinary source inheritance
// continues through the caller's class hierarchy lookup.
// Native API 62/67: ../apextest/testdata/conformance/address_location_review/org.tsv.
func (vm *VM) addressLocationTypesMatch(from, to string) (matched, handled bool) {
	platform := func(name string) (root string, system bool) {
		for _, short := range []string{"Address", "Location"} {
			if strings.EqualFold(name, "System."+short) {
				return short, true
			}
			if strings.EqualFold(name, short) {
				class, found := vm.lookupClass(name)
				return short, !found || class.PlatformType
			}
		}
		return "", false
	}
	source, sourceSystem := platform(from)
	target, targetSystem := platform(to)
	if source == "" || target == "" || (!sourceSystem && !targetSystem) {
		return false, false
	}
	return sourceSystem && targetSystem && (source == target || source == "Address" && target == "Location"), true
}
