package vm

import "strings"

// Named precision observations distinguish Datetime.equals and scalar
// assertions from == and ordering: only the former compare whole seconds.
func datetimeMethodEquals(left, right Value) (bool, bool) {
	if left.Kind != ValueObject || right.Kind != ValueObject ||
		left.projectClass || right.projectClass || left.classInstance || right.classInstance ||
		!strings.EqualFold(left.Type, "Datetime") || !strings.EqualFold(right.Type, "Datetime") {
		return false, false
	}
	leftTime, leftErr := parsePlatformDatetime(left)
	rightTime, rightErr := parsePlatformDatetime(right)
	if leftErr != nil || rightErr != nil {
		return false, false
	}
	// Owned utility snapshots at API62/67/68, anonymous and @IsTest, also
	// observe equality across zero for negative fractional epoch seconds.
	return leftTime.UnixMilli()/1000 == rightTime.UnixMilli()/1000, true
}
