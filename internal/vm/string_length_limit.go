package vm

const apexStringAllocationLimit = 10_000_000

func stringRepeatLengthError(text string, count int64) error {
	length := apexStringLength(text)
	if length > 0 && count > int64(apexStringAllocationLimit/length) {
		return &RuntimeError{Type: "System.LimitException", Message: "String is too long."}
	}
	return nil
}

func concatApexStrings(left, right string) (Value, error) {
	// UTF-8 byte lengths bound UTF-16 lengths, keeping ordinary concatenation
	// on its constant-time path.
	if len(left) > apexStringAllocationLimit-len(right) && apexStringLength(left) > apexStringAllocationLimit-apexStringLength(right) {
		return Null, newExceptionError("StringException", "String length exceeds maximum: 10000000")
	}
	return String(left + right), nil
}
