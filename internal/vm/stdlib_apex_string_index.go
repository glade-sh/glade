package vm

import (
	"fmt"
	"strings"
	"unicode/utf16"
)

func apexStringLength(text string) int {
	count := 0
	for _, r := range text {
		if r <= 0xffff {
			count++
			continue
		}
		count += 2
	}
	return count
}

func apexStringUTF16Units(text string) []uint16 {
	return utf16.Encode([]rune(text))
}

func byteIndexForApexStringIndex(input string, unitIndex int) (int, error) {
	if unitIndex < 0 {
		return 0, fmt.Errorf("string index must be non-negative")
	}
	units := 0
	for byteIndex, r := range input {
		if units == unitIndex {
			return byteIndex, nil
		}
		width := 1
		if r > 0xffff {
			width = 2
		}
		if unitIndex > units && unitIndex < units+width {
			return 0, fmt.Errorf("string index splits a surrogate pair")
		}
		units += width
	}
	if units == unitIndex {
		return len(input), nil
	}
	return 0, fmt.Errorf("string index out of range")
}

func codePointAtApexIndex(text string, index int) (int, error) {
	units := apexStringUTF16Units(text)
	if index < 0 || index >= len(units) {
		return 0, fmt.Errorf("string index out of range")
	}
	first := units[index]
	if utf16.IsSurrogate(rune(first)) && first >= 0xd800 && first <= 0xdbff && index+1 < len(units) {
		second := units[index+1]
		if second >= 0xdc00 && second <= 0xdfff {
			return int(utf16.DecodeRune(rune(first), rune(second))), nil
		}
	}
	return int(first), nil
}

func codePointBeforeApexIndex(text string, index int) (int, error) {
	units := apexStringUTF16Units(text)
	if index <= 0 || index > len(units) {
		return 0, fmt.Errorf("string index out of range")
	}
	last := units[index-1]
	if last >= 0xdc00 && last <= 0xdfff && index >= 2 {
		first := units[index-2]
		if first >= 0xd800 && first <= 0xdbff {
			return int(utf16.DecodeRune(rune(first), rune(last))), nil
		}
	}
	return int(last), nil
}

func codePointCountForApexRange(text string, begin, end int) (int, error) {
	units := apexStringUTF16Units(text)
	if begin < 0 || end < begin || end > len(units) {
		return 0, fmt.Errorf("string index out of range")
	}
	count := 0
	for i := begin; i < end; {
		if units[i] >= 0xd800 && units[i] <= 0xdbff && i+1 < end && units[i+1] >= 0xdc00 && units[i+1] <= 0xdfff {
			i += 2
		} else {
			i++
		}
		count++
	}
	return count, nil
}

func offsetApexIndexByCodePoints(text string, index, offset int) (int, error) {
	units := apexStringUTF16Units(text)
	if index < 0 || index > len(units) {
		return 0, fmt.Errorf("string index out of range")
	}
	pos := index
	if offset >= 0 {
		for remaining := offset; remaining > 0; remaining-- {
			if pos >= len(units) {
				return 0, fmt.Errorf("string index out of range")
			}
			if units[pos] >= 0xd800 && units[pos] <= 0xdbff && pos+1 < len(units) && units[pos+1] >= 0xdc00 && units[pos+1] <= 0xdfff {
				pos += 2
			} else {
				pos++
			}
		}
		return pos, nil
	}
	for remaining := -offset; remaining > 0; remaining-- {
		if pos <= 0 {
			return 0, fmt.Errorf("string index out of range")
		}
		if pos >= 2 && units[pos-1] >= 0xdc00 && units[pos-1] <= 0xdfff && units[pos-2] >= 0xd800 && units[pos-2] <= 0xdbff {
			pos -= 2
		} else {
			pos--
		}
	}
	return pos, nil
}

func apexStringIndexForByteIndex(input string, targetByte int) (int, error) {
	if targetByte < 0 || targetByte > len(input) {
		return 0, fmt.Errorf("byte index out of range")
	}
	units := 0
	for byteIndex, r := range input {
		if byteIndex == targetByte {
			return units, nil
		}
		if byteIndex > targetByte {
			return 0, fmt.Errorf("byte index is not a rune boundary")
		}
		if r <= 0xffff {
			units++
			continue
		}
		units += 2
	}
	if targetByte == len(input) {
		return units, nil
	}
	return 0, fmt.Errorf("byte index is not a rune boundary")
}

func apexStringFromCharArray(values []Value) (string, error) {
	units := make([]uint16, 0, len(values))
	for _, item := range values {
		if item.Kind != ValueInt || item.Int < 0 {
			return "", fmt.Errorf("String.fromCharArray expects non-negative Integer UTF-16 units")
		}
		// Salesforce truncates each Integer to its low UTF-16 code unit.
		units = append(units, uint16(item.Int&0xffff))
	}
	var b strings.Builder
	for _, r := range utf16.Decode(units) {
		b.WriteRune(r)
	}
	return b.String(), nil
}
