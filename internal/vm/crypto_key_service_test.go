package vm

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// api68_native crypto-ec EC-ERRORS-67/68 and crypto-jwe JWE-* rows: without
// PlatformEncryption the EC key and JWE calls raise the captured
// SecurityException before argument validation. A PlatformEncryption org runs
// the enabled surface (crypto_ec_jwe.go): the same calls reach argument
// validation instead of the gate.
func TestCryptoKeyServiceGateFollowsPlatformEncryption(t *testing.T) {
	calls := []string{
		"Crypto.generateKeyPair('S68_NOT_AN_ALGORITHM');",
		"Crypto.getPrivateKey(EncodingUtil.convertFromHex('000000'));",
		"Crypto.parseJwe(Blob.valueOf('a.b.c'));",
		"Blob d = Blob.valueOf('d'); Crypto.encrypt('ECDH-ES+A128KW', 'A128GCM', d, d, d, d, d, d, d);",
		"Blob d = Blob.valueOf('d'); Crypto.decrypt('ECDH-ES+A128KW', 'A128GCM', d, d, d, d, d, d, d);",
	}
	for _, enabled := range []bool{false, true} {
		for _, source := range calls {
			machine := New(io.Discard)
			org := storage.NewOrgState()
			if enabled {
				org.Features = append(org.Features, "PlatformEncryption")
			}
			machine.SetOrg(&org)
			program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "68.0"})
			if err != nil {
				t.Fatal(err)
			}
			_, err = machine.Execute(program)
			var runtimeErr *RuntimeError
			if !errors.As(err, &runtimeErr) {
				t.Fatalf("%s enabled=%t: %v", source, enabled, err)
			}
			if enabled {
				if runtimeErr.ExceptionMessage() == cryptoKeyServiceUnavailableMessage || runtimeErr.Type == "UnsupportedFeature" && !strings.Contains(runtimeErr.Message, "Crypto.") {
					t.Fatalf("%s with PlatformEncryption = %s: %s", source, runtimeErr.Type, runtimeErr.Message)
				}
				continue
			}
			if !strings.HasSuffix(runtimeErr.Type, "SecurityException") || runtimeErr.ExceptionMessage() != cryptoKeyServiceUnavailableMessage {
				t.Fatalf("%s = %s: %s", source, runtimeErr.Type, runtimeErr.ExceptionMessage())
			}
		}
	}
}

// The four-argument AES overloads are not gated (crypto-ec CR-SIZE-67/68).
func TestCryptoKeyServiceGateLeavesAESOverloads(t *testing.T) {
	machine := New(io.Discard)
	org := storage.NewOrgState()
	machine.SetOrg(&org)
	program, err := CompileAnonymousWithOptions("Blob k = Crypto.generateAesKey(256); Blob iv = Crypto.generateAesKey(128); Blob c = Crypto.encrypt('AES256', k, iv, Blob.valueOf('x')); System.assertEquals('x', Crypto.decrypt('AES256', k, iv, c).toString());", CompileOptions{APIVersion: "68.0"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

// Null or non-Blob JweToken components were not captured; they keep an
// unsupported-call error instead of a guessed toString length.
func TestJweTokenNullComponentIsUnsupported(t *testing.T) {
	machine := New(io.Discard)
	program, err := CompileAnonymousWithOptions("Blob b = Blob.valueOf('b'); new System.JweToken('a', 'e', null, b, b, b, b, b);", CompileOptions{APIVersion: "68.0"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = machine.Execute(program)
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) || runtimeErr.Type != "UnsupportedFeature" {
		t.Fatalf("null component = %v", err)
	}
}

// api68_native_second crypto-ec-shield and crypto-jwe-shield: the enabled
// surface returns the captured key sizes and round-trips every JWE pair.
func TestCryptoEnabledECAndJWERoundTrip(t *testing.T) {
	source := `
Blob pair = Crypto.generateKeyPair('P-384');
System.assertEquals(204, pair.size());
System.assertEquals(80, Crypto.getPrivateKey(pair).size());
System.assertEquals(120, Crypto.getPublicKey(pair).size());
Blob sig = Crypto.sign('ES384', Blob.valueOf('m'), Crypto.getPrivateKey(pair));
System.assertEquals(96, sig.size());
System.assert(Crypto.verify('ES384', Blob.valueOf('m'), sig, Crypto.getPublicKey(pair)));
System.assert(!Crypto.verify('ES384', Blob.valueOf('n'), sig, Crypto.getPublicKey(pair)));
Blob rp = Crypto.generateKeyPair('P-256');
Blob ep = Crypto.generateKeyPair('P-256');
Blob cek = Crypto.generateAesKey(256);
Blob iv = EncodingUtil.convertFromHex('000102030405060708090a0b');
Blob compact = Crypto.encrypt('ECDH-ES+A256KW', 'A256GCM', Crypto.getPublicKey(rp), Crypto.getPrivateKey(ep), Crypto.getPublicKey(ep), cek, iv, Blob.valueOf('S68'), Blob.valueOf(''));
System.assertEquals(331, compact.size());
System.JweToken token = Crypto.parseJwe(compact);
Blob clear = Crypto.decrypt(token.getAlgorithm(), token.getEncryptionAlgorithm(), Crypto.getPrivateKey(rp), token.getEphemeralPublicKey(), token.getEncryptedKey(), token.getIv(), token.getCiphertext(), token.getTag(), token.getAad());
System.assertEquals('S68', clear.toString());
`
	machine := New(io.Discard)
	org := storage.NewOrgState()
	org.Features = append(org.Features, "PlatformEncryption")
	machine.SetOrg(&org)
	program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "68.0"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
