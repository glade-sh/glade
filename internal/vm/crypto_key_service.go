package vm

import (
	"fmt"
	"strings"
)

// cryptoKeyServiceUnavailableMessage is the SecurityException text the API 68
// capture (apextest testdata/conformance/api68_native crypto-ec, crypto-jwe)
// records for the EC key and JWE methods in a Developer scratch org created
// without the PlatformEncryption feature, at source APIs 67.0 and 68.0.
const cryptoKeyServiceUnavailableMessage = "This crypto method is not available. Contact your Salesforce administrator."

// cryptoKeyServiceGate rejects the captured EC key and JWE calls before any
// argument validation: an invalid algorithm alias, a truncated key blob and a
// three-part token all fail with the same text. Orgs that declare the
// PlatformEncryption feature run the key and JWE surface in crypto_ec_jwe.go.
func (vm *VM) cryptoKeyServiceGate(callee string, args []Value) (Value, error) {
	if !vm.cryptoEncryptionEnabled() {
		return Null, newExceptionError("System.SecurityException", cryptoKeyServiceUnavailableMessage)
	}
	switch callee {
	case "Crypto.generateKeyPair":
		return vm.cryptoGenerateKeyPair(args)
	case "Crypto.getPrivateKey", "Crypto.getPublicKey":
		return vm.cryptoGetKeyPart(callee, args)
	case "Crypto.parseJwe":
		return vm.cryptoParseJwe(args)
	case "Crypto.encrypt":
		return vm.cryptoJweEncrypt(args)
	case "Crypto.decrypt":
		return vm.cryptoJweDecrypt(args)
	}
	return Null, unsupportedCallError(callee + " local key, certificate, encryption, and random surfaces")
}

// System.JweToken is a value container: the 8-argument constructor stores the
// components and the getters return them (api68_native JWE-CONTAINER-67/68).
const jweTokenType = "System.JweToken"

var jweTokenFields = []string{"algorithm", "encryptionAlgorithm", "ephemeralPublicKey", "encryptedKey", "iv", "ciphertext", "tag", "aad"}

func isJweTokenType(typeName string) bool {
	return strings.EqualFold(typeName, jweTokenType) || strings.EqualFold(typeName, "JweToken")
}

func constructJweToken(args []Value, namedArgs map[string]Value) (Value, error) {
	if len(namedArgs) != 0 || len(args) != len(jweTokenFields) {
		return Null, fmt.Errorf("System.JweToken constructor expects algorithm, encryption algorithm and six Blob components")
	}
	token := Object(jweTokenType)
	for i, field := range jweTokenFields {
		arg := args[i]
		// The capture covers String algorithms and non-null Blobs only.
		if i < 2 && arg.Kind != ValueString || i >= 2 && (arg.Kind != ValueObject || arg.Type != "Blob") {
			return Null, unsupportedCallError("System.JweToken constructor with null or non-Blob components")
		}
		token.Fields[field] = arg
	}
	return token, nil
}

func callJweTokenMember(receiver Value, method string, args []Value) (Value, bool, error) {
	if len(args) != 0 {
		return Null, false, nil
	}
	switch strings.ToLower(method) {
	case "getalgorithm":
		return receiver.Fields["algorithm"], true, nil
	case "getencryptionalgorithm":
		return receiver.Fields["encryptionAlgorithm"], true, nil
	case "getephemeralpublickey":
		return receiver.Fields["ephemeralPublicKey"], true, nil
	case "getencryptedkey":
		return receiver.Fields["encryptedKey"], true, nil
	case "getiv":
		return receiver.Fields["iv"], true, nil
	case "getciphertext":
		return receiver.Fields["ciphertext"], true, nil
	case "gettag":
		return receiver.Fields["tag"], true, nil
	case "getaad":
		return receiver.Fields["aad"], true, nil
	case "tostring":
		return String(jweTokenString(receiver)), true, nil
	}
	return Null, false, nil
}

// jweTokenString renders the captured toString shape: algorithm names and the
// byte length of each Blob component.
func jweTokenString(token Value) string {
	length := func(field string) int {
		return len(token.Fields[field].Fields["value"].String())
	}
	return fmt.Sprintf("System.JweToken[alg=%s, enc=%s, epkLen=%d, encKeyLen=%d, ivLen=%d, ciphertextLen=%d, tagLen=%d, aadLen=%d]",
		token.Fields["algorithm"].Text, token.Fields["encryptionAlgorithm"].Text,
		length("ephemeralPublicKey"), length("encryptedKey"), length("iv"), length("ciphertext"), length("tag"), length("aad"))
}
