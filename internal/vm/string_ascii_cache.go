package vm

import "strings"

const stringASCIIMaxBytes = 4 << 20
const stringASCIIMaxEntries = 8

type stringASCIIEntry struct {
	text  string
	ascii bool
}

// Strings are immutable. Retain a bounded set of classifications so indexed
// scans do not recheck the entire receiver on every character or delimiter.
// A slice avoids hashing the entire string for each cache lookup.
func (vm *VM) stringIsASCII(text string) bool {
	for _, entry := range vm.stringASCII {
		if entry.text == text {
			return entry.ascii
		}
	}
	ascii := asciiPrefixLen(text, len(text)) == len(text)
	if len(text) >= 256 && len(text) <= stringASCIIMaxBytes {
		if len(vm.stringASCII) >= stringASCIIMaxEntries || vm.stringASCIIBytes+len(text) > stringASCIIMaxBytes {
			clear(vm.stringASCII)
			vm.stringASCII = vm.stringASCII[:0]
			vm.stringASCIIBytes = 0
		}
		vm.stringASCII = append(vm.stringASCII, stringASCIIEntry{text, ascii})
		vm.stringASCIIBytes += len(text)
	}
	return ascii
}

func (vm *VM) callCachedStringMember(receiver Value, method string, args []Value) (Value, bool, error) {
	if strings.EqualFold(method, "length") || strings.EqualFold(method, "charAt") || strings.EqualFold(method, "indexOf") || strings.EqualFold(method, "substring") {
		return callStringMemberWithASCII(receiver, method, args, vm.stringIsASCII(receiver.Text))
	}
	return Null, false, nil
}

func stringIndexOfASCII(text, needle string, start int) int {
	if start < 0 {
		start = 0
	}
	if start > len(text) {
		if needle == "" {
			return len(text)
		}
		return -1
	}
	index := strings.Index(text[start:], needle)
	if index < 0 {
		return -1
	}
	return start + index
}
