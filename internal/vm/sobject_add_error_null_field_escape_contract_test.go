package vm

import "testing"

// Retained API67 SObject primary: 59984 bytes, SHA256
// 2bcfb12be7e3a678ad75f59840da6317eca5cfd5535c3860559885d4415c189b.
// String null clause: lines 504-505; Schema.SObjectField: lines 571-572.
// Complete three-parameter signatures: lines 478-479 and 544-545.
// Corresponding Name examples: lines 529-534 and 596-603.
// Catalog 9d96c8f2472acb64d9d2860ab22ebb1e526edccf137b659c2bca441b1ce86a7b:
// /documents/2216/members/5 (Schema.SObjectField,String,Boolean) and
// /documents/2216/members/7 (String,String,Boolean).
// Eight behavior programs contain sixteen authored status/count predicates.
// Two separate typed-null construction programs contain two validity assertions.
// testDataOrg supplies public Account/String Name without records. Each program
// creates a fresh Account and adds at most one non-null plain-text message.
// Both Boolean arguments are call inputs; no HTML/escaping oracle is asserted.
// No field-association list, error message/identity/catchability, DML, repeat,
// alias, native/API interval/AC7 or whole-family credit. Current9.87 is separate.
func TestExecSObjectAddErrorNullFieldEscapeAPI67(t *testing.T) {
	cases := []struct {
		name   string
		source string
	}{
		{name: "typedNullStringConstructionValidity", source: `Account record=new Account(Name='TestAccount');String fieldName=null;System.assertEquals(null,fieldName);record=null;fieldName=null;`},
		{name: "typedNullFieldTokenConstructionValidity", source: `Account record=new Account(Name='TestAccount');Schema.SObjectField fieldToken=null;System.assertEquals(null,fieldToken);record=null;fieldToken=null;`},
		{name: "nameStringEscapeFalseControl", source: `Account record=new Account(Name='TestAccount');String fieldName='Name';record.addError(fieldName,'error is name field',false);System.assertEquals(true,record.hasErrors());System.assertEquals(1,record.getErrors().size());record=null;fieldName=null;`},
		{name: "nameStringEscapeTrueControl", source: `Account record=new Account(Name='TestAccount');String fieldName='Name';record.addError(fieldName,'error is name field',true);System.assertEquals(true,record.hasErrors());System.assertEquals(1,record.getErrors().size());record=null;fieldName=null;`},
		{name: "nameFieldTokenEscapeFalseControl", source: `Account record=new Account(Name='TestAccount');Schema.DescribeFieldResult nameDesc=Account.Name.getDescribe();Schema.SObjectField fieldToken=nameDesc.getSObjectField();record.addError(fieldToken,'error is name field',false);System.assertEquals(true,record.hasErrors());System.assertEquals(1,record.getErrors().size());record=null;fieldToken=null;nameDesc=null;`},
		{name: "nameFieldTokenEscapeTrueControl", source: `Account record=new Account(Name='TestAccount');Schema.DescribeFieldResult nameDesc=Account.Name.getDescribe();Schema.SObjectField fieldToken=nameDesc.getSObjectField();record.addError(fieldToken,'error is name field',true);System.assertEquals(true,record.hasErrors());System.assertEquals(1,record.getErrors().size());record=null;fieldToken=null;nameDesc=null;`},
		{name: "nullStringEscapeFalse", source: `Account record=new Account(Name='TestAccount');String fieldName=null;record.addError(fieldName,'error is sobject',false);System.assertEquals(true,record.hasErrors());System.assertEquals(1,record.getErrors().size());record=null;fieldName=null;`},
		{name: "nullStringEscapeTrue", source: `Account record=new Account(Name='TestAccount');String fieldName=null;record.addError(fieldName,'error is sobject',true);System.assertEquals(true,record.hasErrors());System.assertEquals(1,record.getErrors().size());record=null;fieldName=null;`},
		{name: "nullFieldTokenEscapeFalse", source: `Account record=new Account(Name='TestAccount');Schema.SObjectField fieldToken=null;record.addError(fieldToken,'error is sobject',false);System.assertEquals(true,record.hasErrors());System.assertEquals(1,record.getErrors().size());record=null;fieldToken=null;`},
		{name: "nullFieldTokenEscapeTrue", source: `Account record=new Account(Name='TestAccount');Schema.SObjectField fieldToken=null;record.addError(fieldToken,'error is sobject',true);System.assertEquals(true,record.hasErrors());System.assertEquals(1,record.getErrors().size());record=null;fieldToken=null;`},
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
