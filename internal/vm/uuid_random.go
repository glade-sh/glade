package vm

import (
	"crypto/rand"
	binaryencoding "encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

func (vm *VM) uuidRandomSource() io.Reader {
	if vm.uuidRandomReader != nil {
		return vm.uuidRandomReader
	}
	return rand.Reader
}

func (vm *VM) randomUUID() (Value, error) {
	var randomBytes [16]byte
	if _, err := io.ReadFull(vm.uuidRandomSource(), randomBytes[:]); err != nil {
		return Null, fmt.Errorf("UUID.randomUUID secure random read: %w", err)
	}
	randomBytes[6] = (randomBytes[6] & 0x0f) | 0x40
	randomBytes[8] = (randomBytes[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(randomBytes[:])
	text := encoded[:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:]
	return uuidValue(text), nil
}

// UUID hashes fold the four 32-bit words, including both halves of each 64-bit component.
func uuidHashCode(value Value) int64 {
	text, ok := platformScalarObjectText(value)
	if !ok {
		return 0
	}
	bytes, err := hex.DecodeString(strings.ReplaceAll(text, "-", ""))
	if err != nil || len(bytes) != 16 {
		return 0
	}
	var hash uint32
	for i := 0; i < len(bytes); i += 4 {
		hash ^= binaryencoding.BigEndian.Uint32(bytes[i : i+4])
	}
	return int64(int32(hash))
}
