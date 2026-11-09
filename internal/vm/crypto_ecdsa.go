package vm

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/x509"
)

func ecdsaSignatureHash(algorithm string) (crypto.Hash, bool) {
	switch normalizeCryptoAlgorithm(algorithm) {
	case "ECDSASHA1":
		return crypto.SHA1, true
	case "ECDSASHA256":
		return crypto.SHA256, true
	case "ECDSASHA384":
		return crypto.SHA384, true
	case "ECDSASHA512":
		return crypto.SHA512, true
	default:
		return 0, false
	}
}

// signECDSA signs with a PKCS#8 EC private key. A key that does not parse is a
// SecurityException (native stdlib_locale_crypto_c14 schema describe), not a local digest.
func signECDSA(hash crypto.Hash, input, encodedKey []byte) ([]byte, error) {
	parsed, err := x509.ParsePKCS8PrivateKey(encodedKey)
	if err != nil {
		return nil, newExceptionError("System.SecurityException", "Invalid Crypto Key")
	}
	key, ok := parsed.(*ecdsa.PrivateKey)
	if !ok {
		return nil, newExceptionError("System.SecurityException", "Invalid Crypto Key")
	}
	digest := hash.New()
	_, _ = digest.Write(input)
	signature, err := ecdsa.SignASN1(rand.Reader, key, digest.Sum(nil))
	if err != nil {
		return nil, newExceptionError("System.SecurityException", "Invalid Crypto Key")
	}
	return signature, nil
}

func verifyECDSA(hash crypto.Hash, input, signature, encodedKey []byte) (bool, error) {
	parsed, err := x509.ParsePKIXPublicKey(encodedKey)
	if err != nil {
		return false, newExceptionError("System.SecurityException", "Invalid Crypto Key")
	}
	key, ok := parsed.(*ecdsa.PublicKey)
	if !ok {
		return false, newExceptionError("System.SecurityException", "Invalid Crypto Key")
	}
	digest := hash.New()
	_, _ = digest.Write(input)
	return ecdsa.VerifyASN1(key, digest.Sum(nil), signature), nil
}
