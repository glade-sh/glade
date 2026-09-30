package vm

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
)

func rsaSignatureHash(algorithm string) (crypto.Hash, bool) {
	switch normalizeCryptoAlgorithm(algorithm) {
	case "RSA", "RSASHA1":
		return crypto.SHA1, true
	case "RSASHA256":
		return crypto.SHA256, true
	case "RSASHA384":
		return crypto.SHA384, true
	case "RSASHA512":
		return crypto.SHA512, true
	default:
		return 0, false
	}
}

func signRSA(hash crypto.Hash, input, encodedKey []byte) ([]byte, error) {
	parsed, err := x509.ParsePKCS8PrivateKey(encodedKey)
	if err != nil {
		return nil, newExceptionError("System.SecurityException", "Invalid Crypto Key")
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok || key.Validate() != nil {
		return nil, newExceptionError("System.SecurityException", "Invalid Crypto Key")
	}
	digest := hash.New()
	_, _ = digest.Write(input)
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, hash, digest.Sum(nil))
	if err != nil {
		return nil, newExceptionError("System.SecurityException", "Invalid Crypto Key")
	}
	return signature, nil
}

func verifyRSA(hash crypto.Hash, input, signature, encodedKey []byte) (bool, error) {
	parsed, err := x509.ParsePKIXPublicKey(encodedKey)
	if err != nil {
		return false, newExceptionError("System.SecurityException", "Invalid Crypto Key")
	}
	key, ok := parsed.(*rsa.PublicKey)
	if !ok {
		return false, newExceptionError("System.SecurityException", "Invalid Crypto Key")
	}
	digest := hash.New()
	_, _ = digest.Write(input)
	return rsa.VerifyPKCS1v15(key, hash, digest.Sum(nil), signature) == nil, nil
}
