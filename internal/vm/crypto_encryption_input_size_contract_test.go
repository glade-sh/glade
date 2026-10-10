package vm

import "testing"

// API67 CBC plaintext above 1,048,576 bytes must reject for Crypto.encrypt
// and the three-argument Crypto.encryptWithManagedIV. Retained guide
// apex/apex_classes_restful_crypto.md SHA256
// fe816b3793051930f30c6ec5e855825c066e78babe598e82e2bd7f7a144a00dd,
// lines 69-86; AES128 operand definitions at 414-454 and 491-525.
// Retained source catalog SHA256
// 9d96c8f2472acb64d9d2860ab22ebb1e526edccf137b659c2bca441b1ce86a7b,
// /documents/639/members/3,4. Both entry points use a non-null 16-byte ASCII
// key; direct CBC also uses a non-null 16-byte IV. The separate successful
// 1,048,577-byte Blob construction check prevents fixture construction errors
// from satisfying the rejection cases. It is validity, not a fifth behavior
// case or family credit. Accepting baselines clear discarded ciphertexts.
// Small CBC roundtrip controls reuse TestExecCryptoRequiredNullArgumentsAPI67
// and the pattern in TestExecCryptoManagedIVAndSignatureLocalSubset; retained
// CBC fixture core-blob-crypto-partial-encrypt-unsupported-decrypt-sign-verify.json
// SHA256 f144e0d64bf69d5a98c014b7bda7facb02c1f5dffd3db21ca0c7a0738773cce8.
// No exception identity/message/catchability/timing, boundary-adjacent size,
// decryption allowance, GCM/AAD, native/API-interval or child AC7 qualification
// is inferred from these four bounded CBC behavior cases.
func TestExecCryptoEncryptionInputSizeContractAPI67(t *testing.T) {
	t.Run("oversizedBlobFixtureValidity", func(t *testing.T) {
		program, err := CompileAnonymousWithOptions(`System.assertEquals(1048577,Blob.valueOf('x'.repeat(1048577)).size());`, CompileOptions{APIVersion: "67.0"})
		if err != nil {
			t.Fatal(err)
		}
		if program.APIVersion != "67.0" {
			t.Fatalf("compiled API version = %q, want 67.0", program.APIVersion)
		}
		if _, err := New(nil).Execute(program); err != nil {
			t.Fatal(err)
		}
	})
	cases := []struct {
		name      string
		source    string
		wantError bool
	}{
		{name: "directOversizedPlaintext", source: `Blob discarded=Crypto.encrypt('AES128',Blob.valueOf('0123456789abcdef'),Blob.valueOf('abcdef9876543210'),Blob.valueOf('x'.repeat(1048577)));discarded=null;`, wantError: true},
		{name: "managedOversizedPlaintext", source: `Blob discarded=Crypto.encryptWithManagedIV('AES128',Blob.valueOf('0123456789abcdef'),Blob.valueOf('x'.repeat(1048577)));discarded=null;`, wantError: true},
		{name: "directSmallRoundtrip", source: `Blob key=Blob.valueOf('0123456789abcdef');Blob iv=Blob.valueOf('abcdef9876543210');Blob ciphertext=Crypto.encrypt('AES128',key,iv,Blob.valueOf('hello'));System.assertEquals('hello',Crypto.decrypt('AES128',key,iv,ciphertext).toString());`, wantError: false},
		{name: "managedSmallRoundtrip", source: `Blob key=Blob.valueOf('0123456789abcdef');Blob ciphertext=Crypto.encryptWithManagedIV('AES128',key,Blob.valueOf('hello'));System.assertEquals('hello',Crypto.decryptWithManagedIV('AES128',key,ciphertext).toString());`, wantError: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(tc.source, CompileOptions{APIVersion: "67.0"})
			if err != nil {
				t.Fatal(err)
			}
			if program.APIVersion != "67.0" {
				t.Fatalf("compiled API version = %q, want 67.0", program.APIVersion)
			}
			_, err = New(nil).Execute(program)
			if (err != nil) != tc.wantError {
				t.Fatalf("execution error = %v, want error existence %v", err, tc.wantError)
			}
		})
	}
}
