package vm

import "testing"

// API67 required-null encryption/decryption arguments throw NullPointerException.
// Crypto guide apex/apex_classes_restful_crypto.md SHA256
// fe816b3793051930f30c6ec5e855825c066e78babe598e82e2bd7f7a144a00dd,
// lines 69-87; catalog SHA256
// 9d96c8f2472acb64d9d2860ab22ebb1e526edccf137b659c2bca441b1ce86a7b,
// /documents/639/members/0, /documents/639/members/1 and
// /documents/639/members/3. The exact accepted-red receipt bodies exercise
// encrypt's null key, decrypt's null algorithm and decryptWithManagedIV's
// null ciphertext. A broad Exception catch cannot satisfy the type witness.
// The control is AES128 CBC with a 16-byte key, 16-byte IV and plaintext hello;
// it checks roundtrip plaintext, without a ciphertext or GCM predicate.
// No exception message, wrong-type policy, IV-null policy or full-family
// qualification is inferred by this bounded regression.
func TestExecCryptoRequiredNullArgumentsAPI67(t *testing.T) {
	cases := []struct {
		name   string
		source string
	}{
		{name: "nullKey", source: `Blob missingKey=null;Boolean caught=false;try{Crypto.encrypt('AES128',missingKey,Blob.valueOf('abcdef9876543210'),Blob.valueOf('hello'));}catch(NullPointerException e){caught=true;}catch(Exception e){}System.assertEquals(true,caught);`},
		{name: "nullAlgorithm", source: `Blob key=Blob.valueOf('0123456789abcdef');Blob iv=Blob.valueOf('abcdef9876543210');Blob ciphertext=Crypto.encrypt('AES128',key,iv,Blob.valueOf('hello'));String missingAlgorithm=null;Boolean caught=false;try{Crypto.decrypt(missingAlgorithm,key,iv,ciphertext);}catch(NullPointerException e){caught=true;}catch(Exception e){}System.assertEquals(true,caught);`},
		{name: "nullManagedCiphertext", source: `Blob missingCiphertext=null;Boolean caught=false;try{Crypto.decryptWithManagedIV('AES128',Blob.valueOf('0123456789abcdef'),missingCiphertext);}catch(NullPointerException e){caught=true;}catch(Exception e){}System.assertEquals(true,caught);`},
		{name: "CBCControl", source: `Blob key=Blob.valueOf('0123456789abcdef');Blob iv=Blob.valueOf('abcdef9876543210');Blob ciphertext=Crypto.encrypt('AES128',key,iv,Blob.valueOf('hello'));System.assertEquals('hello',Crypto.decrypt('AES128',key,iv,ciphertext).toString());`},
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
			if _, err := New(nil).Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
