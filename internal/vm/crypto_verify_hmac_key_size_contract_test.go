package vm

import "testing"

// API67 Crypto.verifyHMac privateKey cannot exceed 4 KB: retained guide
// apex/apex_classes_restful_crypto.md SHA256
// fe816b3793051930f30c6ec5e855825c066e78babe598e82e2bd7f7a144a00dd,
// lines 1419-1461, especially 1445-1449; matched-control example 1471-1483.
// Retained source catalog SHA256
// 9d96c8f2472acb64d9d2860ab22ebb1e526edccf137b659c2bca441b1ce86a7b,
// /documents/639/members/17. The 5000 ASCII-byte key exceeds either 4000 or
// 4096 bytes; no exact boundary, exception identity/message/catchability or
// phase predicate is asserted. The separate successful construction control
// checks key size 5000 and MAC size 32; it is fixture validity, not a third
// MAC behavior case or family credit.
// The 32-byte MAC for message/key is retained from core-blob-crypto-stdlib.json
// SHA256 5103161d7ad3fc3e0e5ce2b019bc11cdaa5083c739cb59e577f84f971bc1af7b
// and TestExecBlobEncodingCryptoStdlib. It is a valid comparison operand;
// no claim is made that it matches the oversized key. Local reuse grants no
// native/API-interval, child AC7 or whole-family qualification. Shared MAC/TOTP
// behavior is outside this bounded verifyHMac dispatch contract.
func TestExecCryptoVerifyHmacKeySizeContractAPI67(t *testing.T) {
	t.Run("keyAndMacFixtureValidity", func(t *testing.T) {
		program, err := CompileAnonymousWithOptions(`Blob key=Blob.valueOf('a'.repeat(5000));Blob mac=EncodingUtil.convertFromHex('6e9ef29b75fffc5b7abae527d58fdadb2fe42e7219011976917343065f58ed4a');System.assertEquals(5000,key.size());System.assertEquals(32,mac.size());`, CompileOptions{APIVersion: "67.0"})
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
		{name: "oversizedASCIIKey", source: `Blob key=Blob.valueOf('a'.repeat(5000));Blob mac=EncodingUtil.convertFromHex('6e9ef29b75fffc5b7abae527d58fdadb2fe42e7219011976917343065f58ed4a');Crypto.verifyHMac('HmacSHA256',Blob.valueOf('message'),key,mac);`, wantError: true},
		{name: "retainedThreeByteKey", source: `Blob mac=EncodingUtil.convertFromHex('6e9ef29b75fffc5b7abae527d58fdadb2fe42e7219011976917343065f58ed4a');System.assertEquals(true,Crypto.verifyHMac('HmacSHA256',Blob.valueOf('message'),Blob.valueOf('key'),mac));`, wantError: false},
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
