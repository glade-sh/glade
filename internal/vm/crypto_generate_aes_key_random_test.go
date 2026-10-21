package vm

import "testing"

func TestCryptoGenerateAESKeyReturnsRandomKeysAndPreservesInvalidSize(t *testing.T) {
	program, err := CompileAnonymous(`
for (Integer size : new List<Integer>{128, 192, 256}) {
    Blob first = Crypto.generateAESKey(size);
    Blob second = Crypto.generateAESKey(size);
    System.assertEquals(size / 8, first.size());
    System.assertEquals(size / 8, second.size());
    System.assertNotEquals(EncodingUtil.convertToHex(first), EncodingUtil.convertToHex(second));
}
Boolean invalidSizeRejected = false;
try {
    Crypto.generateAESKey(64);
} catch (Exception e) {
    invalidSizeRejected = true;
    System.assertEquals('System.InvalidParameterValueException', e.getTypeName());
    System.assertEquals('Invalid size for AES key generation (must be 128, 192, or 256)', e.getMessage());
}
System.assert(invalidSizeRejected);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}
