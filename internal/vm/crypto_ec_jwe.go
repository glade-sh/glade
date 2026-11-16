package vm

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	encbinary "encoding/binary"
	"encoding/json"
	"fmt"
	"hash"
	"math/big"
	"strings"
)

// This file models the PlatformEncryption-enabled EC key and JWE surface that
// the API 68 second-pass capture recorded (apextest
// testdata/conformance/api68_native_second crypto-ec-shield and
// crypto-jwe-shield). Byte layouts come from the captured sizes: a key pair is
// a 4-byte big-endian private key length, a minimal PKCS#8 private key and an
// SPKI public key (162 bytes for P-256). Shapes the capture did not record
// keep the unsupported-call base.

const ecSupportedAliases = "EC-P256, EC-P384, EC-P521, P-256, P-384, P-521, ES256, ES384, ES512"

const jweMaxTokenBytes = 1048576

var (
	oidECPublicKey = asn1.ObjectIdentifier{1, 2, 840, 10045, 2, 1}
	oidP256        = asn1.ObjectIdentifier{1, 2, 840, 10045, 3, 1, 7}
	oidP384        = asn1.ObjectIdentifier{1, 3, 132, 0, 34}
	oidP521        = asn1.ObjectIdentifier{1, 3, 132, 0, 35}
)

func ecCurveForAlias(alias string) (elliptic.Curve, asn1.ObjectIdentifier, bool) {
	switch alias {
	case "ES256", "EC-P256", "P-256":
		return elliptic.P256(), oidP256, true
	case "ES384", "EC-P384", "P-384":
		return elliptic.P384(), oidP384, true
	case "ES512", "EC-P521", "P-521":
		return elliptic.P521(), oidP521, true
	}
	return nil, nil, false
}

func ecCurveOID(curve elliptic.Curve) asn1.ObjectIdentifier {
	switch curve {
	case elliptic.P256():
		return oidP256
	case elliptic.P384():
		return oidP384
	default:
		return oidP521
	}
}

type ecPrivateKeyASN1 struct {
	Version    int
	PrivateKey []byte
}

type ecAlgorithmIdentifier struct {
	Algorithm  asn1.ObjectIdentifier
	Parameters asn1.ObjectIdentifier
}

type ecPKCS8 struct {
	Version    int
	Algorithm  ecAlgorithmIdentifier
	PrivateKey []byte
}

func ecMarshalPrivate(key *ecdsa.PrivateKey) ([]byte, error) {
	size := (key.Curve.Params().BitSize + 7) / 8
	inner, err := asn1.Marshal(ecPrivateKeyASN1{Version: 1, PrivateKey: key.D.FillBytes(make([]byte, size))})
	if err != nil {
		return nil, err
	}
	return asn1.Marshal(ecPKCS8{Algorithm: ecAlgorithmIdentifier{oidECPublicKey, ecCurveOID(key.Curve)}, PrivateKey: inner})
}

func ecInvalidBlob(reason string) error {
	return newExceptionError("System.InvalidParameterValueException", "Invalid EC key pair blob: "+reason)
}

func (vm *VM) cryptoEncryptionEnabled() bool {
	return vm.Org != nil && vm.Org.HasFeature("PlatformEncryption")
}

func cryptoBlobBytes(arg Value) ([]byte, bool) {
	if arg.Kind != ValueObject || arg.Type != "Blob" {
		return nil, false
	}
	return []byte(arg.Fields["value"].String()), true
}

func cryptoBlob(data []byte) Value {
	return platformScalar("Blob", string(data))
}

func (vm *VM) cryptoGenerateKeyPair(args []Value) (Value, error) {
	if len(args) != 1 || args[0].Kind != ValueString {
		return Null, unsupportedCallError("Crypto.generateKeyPair with a non-String algorithm")
	}
	curve, _, ok := ecCurveForAlias(args[0].Text)
	if !ok {
		return Null, newExceptionError("System.InvalidParameterValueException", "Unsupported algorithm. Supported: "+ecSupportedAliases)
	}
	key, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		return Null, newExceptionError("System.SecurityException", err.Error())
	}
	private, err := ecMarshalPrivate(key)
	if err != nil {
		return Null, newExceptionError("System.SecurityException", err.Error())
	}
	public, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return Null, newExceptionError("System.SecurityException", err.Error())
	}
	pair := make([]byte, 4, 4+len(private)+len(public))
	encbinary.BigEndian.PutUint32(pair, uint32(len(private)))
	pair = append(pair, private...)
	pair = append(pair, public...)
	return cryptoBlob(pair), nil
}

// ecSplitPair returns the private and public parts of a key pair blob.
func ecSplitPair(pair []byte) (private, rest []byte, err error) {
	if len(pair) < 4 {
		return nil, nil, ecInvalidBlob("too small to contain length prefix")
	}
	length := int32(encbinary.BigEndian.Uint32(pair))
	if length < 0 || int(length) > len(pair)-4 {
		return nil, nil, ecInvalidBlob("private key length out of range")
	}
	return pair[4 : 4+length], pair[4+length:], nil
}

func (vm *VM) cryptoGetKeyPart(callee string, args []Value) (Value, error) {
	if len(args) != 1 {
		return Null, unsupportedCallError(callee + " with a non-Blob argument")
	}
	pair, ok := cryptoBlobBytes(args[0])
	if !ok {
		return Null, unsupportedCallError(callee + " with a non-Blob argument")
	}
	private, public, err := ecSplitPair(pair)
	if err != nil {
		return Null, err
	}
	if callee == "Crypto.getPrivateKey" {
		return cryptoBlob(private), nil
	}
	if len(public) == 0 {
		return Null, ecInvalidBlob("no public key data after private key")
	}
	return cryptoBlob(public), nil
}

// ES256, ES384 and ES512 sign with a PKCS#8 EC key and return the raw r||s
// signature (64, 96 and 132 bytes in the capture).
func ecSignatureAlgorithm(algorithm string) (hash.Hash, int, bool) {
	switch algorithm {
	case "ES256":
		return sha256.New(), 32, true
	case "ES384":
		return sha512.New384(), 48, true
	case "ES512":
		return sha512.New(), 66, true
	}
	return nil, 0, false
}

func signES(algorithm string, input, encodedKey []byte) ([]byte, error) {
	digest, size, _ := ecSignatureAlgorithm(algorithm)
	parsed, err := x509.ParsePKCS8PrivateKey(encodedKey)
	if err != nil {
		return nil, newExceptionError("System.SecurityException", "Invalid Crypto Key")
	}
	key, ok := parsed.(*ecdsa.PrivateKey)
	if !ok {
		return nil, newExceptionError("System.SecurityException", "Invalid Crypto Key")
	}
	_, _ = digest.Write(input)
	r, s, err := ecdsa.Sign(rand.Reader, key, digest.Sum(nil))
	if err != nil {
		return nil, newExceptionError("System.SecurityException", "Invalid Crypto Key")
	}
	out := make([]byte, 2*size)
	r.FillBytes(out[:size])
	s.FillBytes(out[size:])
	return out, nil
}

func verifyES(algorithm string, input, signature, encodedKey []byte) (bool, error) {
	digest, size, _ := ecSignatureAlgorithm(algorithm)
	parsed, err := x509.ParsePKIXPublicKey(encodedKey)
	if err != nil {
		return false, newExceptionError("System.SecurityException", "Invalid Crypto Key")
	}
	key, ok := parsed.(*ecdsa.PublicKey)
	if !ok {
		return false, newExceptionError("System.SecurityException", "Invalid Crypto Key")
	}
	if len(signature) != 2*size {
		return false, nil
	}
	_, _ = digest.Write(input)
	r := new(big.Int).SetBytes(signature[:size])
	s := new(big.Int).SetBytes(signature[size:])
	return ecdsa.Verify(key, digest.Sum(nil), r, s), nil
}

type jweContentEncryption struct {
	name     string
	cekBytes int
	gcm      bool
	tagBytes int
}

var jweContentEncryptions = []jweContentEncryption{
	{"A128GCM", 16, true, 16},
	{"A192GCM", 24, true, 16},
	{"A256GCM", 32, true, 16},
	{"A128CBC-HS256", 32, false, 16},
	{"A192CBC-HS384", 48, false, 24},
	{"A256CBC-HS512", 64, false, 32},
}

func jweContentEncryptionByName(name string) (jweContentEncryption, bool) {
	for _, candidate := range jweContentEncryptions {
		if candidate.name == name {
			return candidate, true
		}
	}
	return jweContentEncryption{}, false
}

func jweUnsupportedContentEncryption(name string) error {
	names := make([]string, len(jweContentEncryptions))
	for i, candidate := range jweContentEncryptions {
		names[i] = candidate.name
	}
	return newExceptionError("System.SecurityException", "Unsupported content encryption algorithm: "+name+". Supported: "+strings.Join(names, ", "))
}

func jweKeyWrapBytes(algorithm string) (int, bool) {
	switch algorithm {
	case "ECDH-ES+A128KW":
		return 16, true
	case "ECDH-ES+A192KW":
		return 24, true
	case "ECDH-ES+A256KW":
		return 32, true
	}
	return 0, false
}

// jweConcatKDF is the RFC 7518 section 4.6 key derivation with empty
// PartyUInfo and PartyVInfo.
func jweConcatKDF(secret []byte, algorithm string, keyBytes int) []byte {
	var input []byte
	input = encbinary.BigEndian.AppendUint32(input, 1)
	input = append(input, secret...)
	input = encbinary.BigEndian.AppendUint32(input, uint32(len(algorithm)))
	input = append(input, algorithm...)
	input = encbinary.BigEndian.AppendUint32(input, 0)
	input = encbinary.BigEndian.AppendUint32(input, 0)
	input = encbinary.BigEndian.AppendUint32(input, uint32(keyBytes*8))
	sum := sha256.Sum256(input)
	return sum[:keyBytes]
}

var aesKeyWrapIV = []byte{0xA6, 0xA6, 0xA6, 0xA6, 0xA6, 0xA6, 0xA6, 0xA6}

func aesKeyWrap(kek, plain []byte) ([]byte, error) {
	block, err := aes.NewCipher(kek)
	if err != nil {
		return nil, err
	}
	n := len(plain) / 8
	a := append([]byte(nil), aesKeyWrapIV...)
	r := make([][]byte, n)
	for i := range r {
		r[i] = append([]byte(nil), plain[i*8:(i+1)*8]...)
	}
	buf := make([]byte, 16)
	for j := 0; j < 6; j++ {
		for i := 0; i < n; i++ {
			copy(buf, a)
			copy(buf[8:], r[i])
			block.Encrypt(buf, buf)
			t := uint64(n*j + i + 1)
			copy(a, buf[:8])
			encbinary.BigEndian.PutUint64(a, encbinary.BigEndian.Uint64(a)^t)
			copy(r[i], buf[8:])
		}
	}
	out := append([]byte(nil), a...)
	for _, part := range r {
		out = append(out, part...)
	}
	return out, nil
}

func aesKeyUnwrap(kek, wrapped []byte) ([]byte, bool) {
	block, err := aes.NewCipher(kek)
	if err != nil || len(wrapped) < 24 || len(wrapped)%8 != 0 {
		return nil, false
	}
	n := len(wrapped)/8 - 1
	a := append([]byte(nil), wrapped[:8]...)
	r := make([][]byte, n)
	for i := range r {
		r[i] = append([]byte(nil), wrapped[(i+1)*8:(i+2)*8]...)
	}
	buf := make([]byte, 16)
	for j := 5; j >= 0; j-- {
		for i := n - 1; i >= 0; i-- {
			t := uint64(n*j + i + 1)
			copy(buf, a)
			encbinary.BigEndian.PutUint64(buf, encbinary.BigEndian.Uint64(buf)^t)
			copy(buf[8:], r[i])
			block.Decrypt(buf, buf)
			copy(a, buf[:8])
			copy(r[i], buf[8:])
		}
	}
	if subtle.ConstantTimeCompare(a, aesKeyWrapIV) != 1 {
		return nil, false
	}
	var out []byte
	for _, part := range r {
		out = append(out, part...)
	}
	return out, true
}

func jwePKCS7Pad(data []byte, size int) []byte {
	pad := size - len(data)%size
	out := append([]byte(nil), data...)
	for i := 0; i < pad; i++ {
		out = append(out, byte(pad))
	}
	return out
}

func jwePKCS7Unpad(data []byte, size int) ([]byte, bool) {
	if len(data) == 0 || len(data)%size != 0 {
		return nil, false
	}
	pad := int(data[len(data)-1])
	if pad == 0 || pad > size || pad > len(data) {
		return nil, false
	}
	for _, b := range data[len(data)-pad:] {
		if int(b) != pad {
			return nil, false
		}
	}
	return data[:len(data)-pad], true
}

func jweCBCTag(enc jweContentEncryption, macKey, aad, iv, ciphertext []byte) []byte {
	var mac hash.Hash
	switch enc.cekBytes {
	case 32:
		mac = hmac.New(sha256.New, macKey)
	case 48:
		mac = hmac.New(sha512.New384, macKey)
	default:
		mac = hmac.New(sha512.New, macKey)
	}
	mac.Write(aad)
	mac.Write(iv)
	mac.Write(ciphertext)
	var length [8]byte
	encbinary.BigEndian.PutUint64(length[:], uint64(len(aad))*8)
	mac.Write(length[:])
	return mac.Sum(nil)[:enc.tagBytes]
}

var errJweTag = fmt.Errorf("tag mismatch")

func jweSeal(enc jweContentEncryption, cek, iv, aad, plain []byte) (ciphertext, tag []byte, err error) {
	if enc.gcm {
		block, err := aes.NewCipher(cek)
		if err != nil {
			return nil, nil, err
		}
		aead, err := cipher.NewGCMWithNonceSize(block, len(iv))
		if err != nil {
			return nil, nil, err
		}
		sealed := aead.Seal(nil, iv, plain, aad)
		return sealed[:len(plain)], sealed[len(plain):], nil
	}
	half := enc.cekBytes / 2
	block, err := aes.NewCipher(cek[half:])
	if err != nil {
		return nil, nil, err
	}
	padded := jwePKCS7Pad(plain, aes.BlockSize)
	ciphertext = make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(ciphertext, padded)
	return ciphertext, jweCBCTag(enc, cek[:half], aad, iv, ciphertext), nil
}

func jweOpen(enc jweContentEncryption, cek, iv, aad, ciphertext, tag []byte) ([]byte, error) {
	if enc.gcm {
		block, err := aes.NewCipher(cek)
		if err != nil {
			return nil, err
		}
		aead, err := cipher.NewGCMWithNonceSize(block, len(iv))
		if err != nil {
			return nil, errJweTag
		}
		plain, err := aead.Open(nil, iv, append(append([]byte(nil), ciphertext...), tag...), aad)
		if err != nil {
			return nil, errJweTag
		}
		return plain, nil
	}
	half := enc.cekBytes / 2
	if subtle.ConstantTimeCompare(jweCBCTag(enc, cek[:half], aad, iv, ciphertext), tag) != 1 {
		return nil, errJweTag
	}
	block, err := aes.NewCipher(cek[half:])
	if err != nil || len(iv) != aes.BlockSize || len(ciphertext)%aes.BlockSize != 0 {
		return nil, errJweTag
	}
	plain := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plain, ciphertext)
	unpadded, ok := jwePKCS7Unpad(plain, aes.BlockSize)
	if !ok {
		return nil, errJweTag
	}
	return unpadded, nil
}

func jweEncryptionFailed(detail string) error {
	return newExceptionError("System.SecurityException", "Encryption failed: "+detail)
}

func jweDecryptionFailed(detail string) error {
	return newExceptionError("System.SecurityException", "Decryption failed: "+detail)
}

var jweUndecodableKey = jweEncryptionFailed("java.security.InvalidKeyException: Unable to decode key")

func ecPrivateToECDH(encoded []byte) (*ecdh.PrivateKey, bool) {
	parsed, err := x509.ParsePKCS8PrivateKey(encoded)
	if err != nil {
		return nil, false
	}
	key, ok := parsed.(*ecdsa.PrivateKey)
	if !ok {
		return nil, false
	}
	converted, err := key.ECDH()
	return converted, err == nil
}

func ecPublicToECDH(encoded []byte) (*ecdh.PublicKey, bool) {
	parsed, err := x509.ParsePKIXPublicKey(encoded)
	if err != nil {
		return nil, false
	}
	key, ok := parsed.(*ecdsa.PublicKey)
	if !ok {
		return nil, false
	}
	converted, err := key.ECDH()
	return converted, err == nil
}

func jweBase64URL(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

func jweCurveName(curve elliptic.Curve) string {
	switch curve {
	case elliptic.P256():
		return "P-256"
	case elliptic.P384():
		return "P-384"
	default:
		return "P-521"
	}
}

// vm.cryptoJweEncrypt is Crypto.encrypt(alg, enc, recipientPublicKey,
// ephemeralPrivateKey, ephemeralPublicKey, cek, iv, plaintext, aad). The
// capture accepted exactly this key order.
func (vm *VM) cryptoJweEncrypt(args []Value) (Value, error) {
	if len(args) != 9 || args[0].Kind != ValueString || args[1].Kind != ValueString {
		return Null, unsupportedCallError("Crypto.encrypt JWE with a non-String algorithm")
	}
	blobs := make([][]byte, 7)
	for i := range blobs {
		data, ok := cryptoBlobBytes(args[i+2])
		if !ok {
			return Null, unsupportedCallError("Crypto.encrypt JWE with a null or non-Blob argument")
		}
		blobs[i] = data
	}
	recipientPublic, ephemeralPrivate, ephemeralPublic, cek, iv, plain, aad := blobs[0], blobs[1], blobs[2], blobs[3], blobs[4], blobs[5], blobs[6]
	wrapBytes, ok := jweKeyWrapBytes(args[0].Text)
	enc, encOK := jweContentEncryptionByName(args[1].Text)
	if !ok || !encOK {
		return Null, unsupportedCallError("Crypto.encrypt JWE with an unrecorded algorithm")
	}
	if len(aad) != 0 {
		return Null, newExceptionError("System.InvalidParameterValueException", "Additional AAD isn't supported in JWE compact serialization. Use empty AAD.")
	}
	if len(cek) != enc.cekBytes {
		return Null, newExceptionError("System.InvalidParameterValueException", fmt.Sprintf("CEK length mismatch: expected %d bytes for %s, got %d", enc.cekBytes, enc.name, len(cek)))
	}
	recipient, recipientOK := ecPublicToECDH(recipientPublic)
	private, privateOK := ecPrivateToECDH(ephemeralPrivate)
	public, publicOK := ecPublicToECDH(ephemeralPublic)
	if !recipientOK || !privateOK || !publicOK {
		return Null, jweUndecodableKey
	}
	if private.Curve() != recipient.Curve() || !private.PublicKey().Equal(public) {
		return Null, unsupportedCallError("Crypto.encrypt JWE with mismatched ephemeral or recipient keys")
	}
	if enc.gcm == false && len(iv) != aes.BlockSize {
		return Null, unsupportedCallError("Crypto.encrypt JWE CBC with a non-16-byte IV")
	}
	secret, err := private.ECDH(recipient)
	if err != nil {
		return Null, jweUndecodableKey
	}
	wrapped, err := aesKeyWrap(jweConcatKDF(secret, args[0].Text, wrapBytes), cek)
	if err != nil {
		return Null, jweEncryptionFailed(err.Error())
	}
	parsedPublic, _ := x509.ParsePKIXPublicKey(ephemeralPublic)
	pub := parsedPublic.(*ecdsa.PublicKey)
	size := (pub.Curve.Params().BitSize + 7) / 8
	header := fmt.Sprintf(`{"alg":%q,"enc":%q,"epk":{"kty":"EC","crv":%q,"x":%q,"y":%q}}`,
		args[0].Text, args[1].Text, jweCurveName(pub.Curve),
		jweBase64URL(pub.X.FillBytes(make([]byte, size))), jweBase64URL(pub.Y.FillBytes(make([]byte, size))))
	protected := jweBase64URL([]byte(header))
	ciphertext, tag, err := jweSeal(enc, cek, iv, []byte(protected), plain)
	if err != nil {
		return Null, jweEncryptionFailed(err.Error())
	}
	token := strings.Join([]string{protected, jweBase64URL(wrapped), jweBase64URL(iv), jweBase64URL(ciphertext), jweBase64URL(tag)}, ".")
	return cryptoBlob([]byte(token)), nil
}

// jweDecodePart decodes one base64url compact part.
func jweDecodePart(part string) ([]byte, bool) {
	data, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(part, "="))
	return data, err == nil
}

func jweJSONFailure(text string) error {
	// The capture records Jackson's message for a bare word.
	end := 0
	for end < len(text) && (text[end] >= 'a' && text[end] <= 'z' || text[end] >= 'A' && text[end] <= 'Z') {
		end++
	}
	if end == 0 {
		return unsupportedCallError("Crypto.parseJwe with an unrecorded invalid header")
	}
	return newExceptionError("System.SecurityException", "JWE parsing failed: Unrecognized token '"+text[:end]+"': was expecting (JSON String, Number, Array, Object or token 'null', 'true' or 'false')")
}

func (vm *VM) cryptoParseJwe(args []Value) (Value, error) {
	if len(args) != 1 {
		return Null, unsupportedCallError("Crypto.parseJwe with a non-Blob argument")
	}
	compact, ok := cryptoBlobBytes(args[0])
	if !ok {
		return Null, unsupportedCallError("Crypto.parseJwe with a non-Blob argument")
	}
	if len(compact) > jweMaxTokenBytes {
		return Null, newExceptionError("System.InvalidParameterValueException", fmt.Sprintf("JWE token exceeds maximum size of %d bytes.", jweMaxTokenBytes))
	}
	parts := strings.Split(string(compact), ".")
	if len(parts) != 5 {
		return Null, newExceptionError("System.SecurityException", fmt.Sprintf("Invalid JWE compact serialization. Expected 5 dot-separated parts, got %d", len(parts)))
	}
	headerBytes, ok := jweDecodePart(parts[0])
	if !ok {
		return Null, unsupportedCallError("Crypto.parseJwe with an unrecorded invalid header encoding")
	}
	var header map[string]json.RawMessage
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return Null, jweJSONFailure(string(headerBytes))
	}
	text := func(key string) (string, bool) {
		var value string
		raw, present := header[key]
		if !present || json.Unmarshal(raw, &value) != nil {
			return "", false
		}
		return value, true
	}
	algorithm, algOK := text("alg")
	if !algOK {
		return Null, newExceptionError("System.SecurityException", "JWE header missing required 'alg' field")
	}
	encName, encOK := text("enc")
	if !encOK {
		return Null, newExceptionError("System.SecurityException", "JWE header missing required `enc` field")
	}
	rawEPK, hasEPK := header["epk"]
	if !hasEPK {
		return Null, newExceptionError("System.SecurityException", "JWE header missing required `epk` (ephemeral public key) field")
	}
	var epk map[string]string
	if json.Unmarshal(rawEPK, &epk) != nil {
		return Null, unsupportedCallError("Crypto.parseJwe with an unrecorded epk shape")
	}
	if epk["kty"] != "EC" {
		return Null, newExceptionError("System.SecurityException", "JWE parsing failed: EPK JWK `kty` must be `EC`, got: "+epk["kty"])
	}
	curve := map[string]elliptic.Curve{"P-256": elliptic.P256(), "P-384": elliptic.P384(), "P-521": elliptic.P521()}[epk["crv"]]
	if epk["crv"] != "" && curve == nil {
		return Null, newExceptionError("System.SecurityException", "JWE parsing failed: Unsupported EPK curve: "+epk["crv"]+". Supported: P-256, P-384, P-521")
	}
	if epk["crv"] == "" || epk["x"] == "" || epk["y"] == "" {
		return Null, newExceptionError("System.SecurityException", "JWE parsing failed: EPK JWK missing required fields (`crv`, `x`, `y`)")
	}
	if _, supported := jweContentEncryptionByName(encName); !supported {
		return Null, jweUnsupportedContentEncryption(encName)
	}
	if _, supported := jweKeyWrapBytes(algorithm); !supported {
		return Null, unsupportedCallError("Crypto.parseJwe with an unrecorded key wrap algorithm")
	}
	xBytes, xOK := jweDecodePart(epk["x"])
	yBytes, yOK := jweDecodePart(epk["y"])
	if !xOK || !yOK {
		return Null, unsupportedCallError("Crypto.parseJwe with an unrecorded epk coordinate encoding")
	}
	x, y := new(big.Int).SetBytes(xBytes), new(big.Int).SetBytes(yBytes)
	if !curve.IsOnCurve(x, y) {
		return Null, unsupportedCallError("Crypto.parseJwe with an epk off the curve")
	}
	epkDER, err := x509.MarshalPKIXPublicKey(&ecdsa.PublicKey{Curve: curve, X: x, Y: y})
	if err != nil {
		return Null, unsupportedCallError("Crypto.parseJwe with an unencodable epk")
	}
	components := [4][]byte{}
	for i := range components {
		data, ok := jweDecodePart(parts[i+1])
		if !ok {
			return Null, unsupportedCallError("Crypto.parseJwe with an unrecorded component encoding")
		}
		components[i] = data
	}
	return constructJweToken([]Value{String(algorithm), String(encName), cryptoBlob(epkDER),
		cryptoBlob(components[0]), cryptoBlob(components[1]), cryptoBlob(components[2]), cryptoBlob(components[3]),
		cryptoBlob([]byte(parts[0]))}, nil)
}

// vm.cryptoJweDecrypt is Crypto.decrypt(alg, enc, recipientPrivateKey,
// ephemeralPublicKey, encryptedKey, iv, ciphertext, tag, aad).
func (vm *VM) cryptoJweDecrypt(args []Value) (Value, error) {
	if len(args) != 9 || args[0].Kind != ValueString || args[1].Kind != ValueString {
		return Null, unsupportedCallError("Crypto.decrypt JWE with a non-String algorithm")
	}
	blobs := make([][]byte, 7)
	for i := range blobs {
		data, ok := cryptoBlobBytes(args[i+2])
		if !ok {
			return Null, unsupportedCallError("Crypto.decrypt JWE with a null or non-Blob argument")
		}
		blobs[i] = data
	}
	recipientPrivate, ephemeralPublic, wrapped, iv, ciphertext, tag, aad := blobs[0], blobs[1], blobs[2], blobs[3], blobs[4], blobs[5], blobs[6]
	wrapBytes, ok := jweKeyWrapBytes(args[0].Text)
	enc, encOK := jweContentEncryptionByName(args[1].Text)
	if !ok || !encOK {
		return Null, unsupportedCallError("Crypto.decrypt JWE with an unrecorded algorithm")
	}
	private, privateOK := ecPrivateToECDH(recipientPrivate)
	public, publicOK := ecPublicToECDH(ephemeralPublic)
	if !privateOK || !publicOK {
		return Null, jweDecryptionFailed("java.security.InvalidKeyException: Unable to decode key")
	}
	if enc.gcm && len(iv) != 12 {
		return Null, jweDecryptionFailed(fmt.Sprintf("AES-GCM IV must be 96 bits (12 bytes), got %d", len(iv)))
	}
	if len(wrapped) == 0 {
		return Null, jweDecryptionFailed("Encrypted key must not be empty for key-wrap algorithms")
	}
	if private.Curve() != public.Curve() {
		return Null, unsupportedCallError("Crypto.decrypt JWE with mismatched curves")
	}
	secret, err := private.ECDH(public)
	if err != nil {
		return Null, jweDecryptionFailed("java.security.InvalidKeyException: Unable to decode key")
	}
	cek, unwrapped := aesKeyUnwrap(jweConcatKDF(secret, args[0].Text, wrapBytes), wrapped)
	if !unwrapped || len(cek) != enc.cekBytes {
		// A failed unwrap continues with a random key, so the failure surfaces
		// as a tag mismatch (a wrong private key in the capture).
		cek = make([]byte, enc.cekBytes)
		if _, err := rand.Read(cek); err != nil {
			return Null, jweDecryptionFailed(err.Error())
		}
	}
	plain, err := jweOpen(enc, cek, iv, aad, ciphertext, tag)
	if err != nil {
		if !enc.gcm {
			return Null, unsupportedCallError("Crypto.decrypt JWE CBC tag failure")
		}
		return Null, jweDecryptionFailed("Tag mismatch")
	}
	return cryptoBlob(plain), nil
}
