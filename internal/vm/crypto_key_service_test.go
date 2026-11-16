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
// SecurityException before argument validation. The enabled service is not
// modeled, so a PlatformEncryption org keeps the unsupported-call error.
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
				if runtimeErr.Type != "UnsupportedFeature" {
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
