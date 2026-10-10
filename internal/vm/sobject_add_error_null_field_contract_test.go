package vm

import "testing"

// SObject.addError's Schema.SObjectField
// overload explicitly allows null and associates the error with the SObject.
// Retained SObject primary SHA256
// 2bcfb12be7e3a678ad75f59840da6317eca5cfd5535c3860559885d4415c189b,
// lines 453-455; the full two-argument block is lines 426-470.
// Catalog 9d96c8f2472acb64d9d2860ab22ebb1e526edccf137b659c2bca441b1ce86a7b,
// /documents/2216/members/4. Its raw signature lacks a closing parenthesis;
// parameter declarations and the two-argument example establish this profile.
// The Name-token example at lines 461-468 supports hasErrors and one error;
// /documents/2216/members/25 and /16 describe hasErrors and getErrors.
// Exactly two behavior cases/four authored predicates plus one separate
// typed-null construction assertion; construction is fixture validity only.
// testDataOrg supplies public Account/String Name with no records. No DML,
// error fields/message/identity/catchability, replacement/alias/order, HTML,
// native/API interval/child AC7 or whole-SObject-family claim is made.
func TestExecSObjectAddErrorNullFieldTokenAPI67(t *testing.T) {
	cases := []struct {
		name   string
		source string
	}{
		{name: "typedNullConstructionValidity", source: `Account record=new Account(Name='TestAccount');Schema.SObjectField fieldToken=null;System.assertEquals(null,fieldToken);record=null;fieldToken=null;`},
		{name: "nameFieldTokenControl", source: `Account record=new Account(Name='TestAccount');Schema.DescribeFieldResult nameDesc=Account.Name.getDescribe();Schema.SObjectField fieldToken=nameDesc.getSObjectField();record.addError(fieldToken,'error is name field');System.assert(record.hasErrors());System.assertEquals(1,record.getErrors().size());record=null;fieldToken=null;nameDesc=null;`},
		{name: "nullFieldToken", source: `Account record=new Account(Name='TestAccount');Schema.SObjectField fieldToken=null;record.addError(fieldToken,'error is sobject');System.assert(record.hasErrors());System.assertEquals(1,record.getErrors().size());record=null;fieldToken=null;`},
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
			org := testDataOrg()
			machine := New(nil)
			machine.SetOrg(&org)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
