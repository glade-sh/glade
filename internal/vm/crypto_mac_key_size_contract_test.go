package vm

import "testing"

// API67 Crypto.generateMac privateKey cannot exceed 4 KB: retained guide
// apex/apex_classes_restful_crypto.md SHA256
// fe816b3793051930f30c6ec5e855825c066e78babe598e82e2bd7f7a144a00dd,
// lines 756-787, especially 781-787. Retained API67 source catalog SHA256
// 9d96c8f2472acb64d9d2860ab22ebb1e526edccf137b659c2bca441b1ce86a7b,
// /documents/639/members/8. The 5000 ASCII-byte key exceeds either 4000 or
// 4096 bytes; no exact boundary or exception identity/message/catchability
// predicate is asserted. A separate successful construction control prevents
// the oversized case from accepting a repeat/key.size construction failure.
// That control is fixture validity, not a third MAC behavior case or family
// credit; the size check within the rejection body earns no assertion credit.
// The three-byte-key MAC control is retained from core-blob-crypto-stdlib.json
// SHA256 5103161d7ad3fc3e0e5ce2b019bc11cdaa5083c739cb59e577f84f971bc1af7b
// and TestExecBlobEncodingCryptoStdlib. Local reuse is not native/API-interval,
// child AC7 or whole-family qualification. Shared MAC/TOTP behavior is outside
// this bounded generateMac dispatch contract.
func TestExecCryptoGenerateMacKeySizeContractAPI67(t *testing.T) {
	t.Run("oversizedKeyFixtureValidity", func(t *testing.T) {
		program, err := CompileAnonymousWithOptions(`Blob key=Blob.valueOf('a'.repeat(5000));System.assertEquals(5000,key.size());`, CompileOptions{APIVersion: "67.0"})
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
		{name: "oversizedASCIIKey", source: `Blob key=Blob.valueOf('a'.repeat(5000));System.assertEquals(5000,key.size());Crypto.generateMac('HmacSHA256',Blob.valueOf('message'),key);`, wantError: true},
		{name: "retainedThreeByteKey", source: `Blob mac=Crypto.generateMac('HmacSHA256',Blob.valueOf('message'),Blob.valueOf('key'));System.assertEquals('6e9ef29b75fffc5b7abae527d58fdadb2fe42e7219011976917343065f58ed4a',EncodingUtil.convertToHex(mac));`, wantError: false},
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
