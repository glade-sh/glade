package vm

import (
	"crypto/subtle"
	"encoding/base32"
	binaryencoding "encoding/binary"
	"fmt"
	"strings"
)

// validateAuthTotpForKey implements the admitted supplied-key argument and
// three-step window and per-user failed-validation contracts. Enrollment,
// cross-transaction resets and durable verification history remain separate.
func (vm *VM) validateAuthTotpForKey(args []Value) (value Value, err error) {
	const callee = "Auth.SessionManagement.validateTotpTokenForKey"
	if len(args) != 2 && len(args) != 3 {
		return Null, fmt.Errorf("%s expects shared key, code, and optional description", callee)
	}
	for _, value := range args {
		if value.Kind != ValueString && value.Kind != ValueNull {
			return Null, unsupportedCallError(callee + " unproved non-String argument")
		}
	}
	if args[0].Kind == ValueNull {
		return Null, newExceptionError("System.InvalidParameterValueException", "Missing shared key")
	}
	// Salesforce accepts the supplied uppercase base32 key with trailing padding.
	// Decode without silently accepting lowercase, whitespace or embedded padding.
	encoded := strings.TrimRight(args[0].Text, "=")
	for _, char := range encoded {
		if !(char >= 'A' && char <= 'Z') && !(char >= '2' && char <= '7') {
			return Null, newExceptionError("System.InvalidParameterValueException", "Invalid shared key")
		}
	}
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(encoded)
	if err != nil || len(key) != 20 {
		return Null, newExceptionError("System.InvalidParameterValueException", "Invalid shared key")
	}
	if args[1].Kind == ValueNull {
		return Null, newExceptionError("System.InvalidParameterValueException", "Missing TOTP code")
	}
	if len(args) == 3 && (args[2].Kind == ValueNull || args[2].Text == "") {
		return Null, newExceptionError("System.InvalidParameterValueException", "Missing description")
	}
	// Argument exceptions precede lockout and do not consume failures. Successful
	// validations neither consume nor reset the user's failed-return count.
	fallbackID := "system"
	if vm.Org != nil {
		fallbackID = "005000000000001"
	}
	userID := displayIDText(vm.currentUserInfoField("Id", fallbackID))
	if vm.authTotpFailures[userID] >= 10 {
		return Null, newExceptionError("System.SecurityException", "Too many token validations for user")
	}
	if vm.authTotpFailures == nil {
		vm.authTotpFailures = make(map[string]int)
	}
	defer func() {
		if err == nil && value.Kind == ValueBool && !value.Bool {
			vm.authTotpFailures[userID]++
		}
	}()
	code := args[1].Text
	if len(code) != 6 {
		return Bool(false), nil
	}
	for i := range code {
		if code[i] < '0' || code[i] > '9' {
			return Bool(false), nil
		}
	}
	// Use the advancing clock shared with Auth JWT claims and cache expiry.
	// lastNow is only a cached Datetime.now return and can outlive clock advances.
	matched := 0
	for step := int64(-1); step <= 1; step++ {
		expected, err := authTotpCode(key, vm.fakeNow.Unix()+step*30)
		if err != nil {
			return Null, err
		}
		matched |= subtle.ConstantTimeCompare([]byte(code), []byte(expected))
	}
	return Bool(matched == 1), nil
}

func authTotpCode(key []byte, seconds int64) (string, error) {
	var counter [8]byte
	binaryencoding.BigEndian.PutUint64(counter[:], uint64(seconds/30))
	mac, err := generateMac("HmacSHA1", counter[:], key)
	if err != nil {
		return "", err
	}
	offset := mac[len(mac)-1] & 15
	truncated := binaryencoding.BigEndian.Uint32(mac[offset:offset+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", truncated%1000000), nil
}
