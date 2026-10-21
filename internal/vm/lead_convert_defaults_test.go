package vm

import "testing"

// The Apex Reference Guide's LeadConvert methods document false defaults for
// isDoNotCreateOpportunity, isOverWriteLeadSource and isSendNotificationEmail.
// https://resources.docs.salesforce.com/latest/latest/en-us/sfdc/pdf/salesforce_apex_reference_guide.pdf
func TestLeadConvertDocumentedBooleanDefaults(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, testContext := range []bool{false, true} {
			route := "anonymous"
			if testContext {
				route = "isTest"
			}
			for _, probe := range []struct{ name, source string }{
				{"defaults", `
Database.LeadConvert instruction = new Database.LeadConvert();
System.assertEquals(false,instruction.isDoNotCreateOpportunity());
System.assertEquals(false,instruction.isOverwriteLeadSource());
System.assertEquals(false,instruction.isSendNotificationEmail());
`},
				{"opportunityBranch", `
Database.LeadConvert instruction = new Database.LeadConvert();
Opportunity draft = ConversionDraftFactory.makeOpportunity(instruction);
System.assertNotEquals(null,draft);
System.assertEquals('Draft',draft.Name);
`},
				{"retainedControls", `
Database.LeadConvert instruction = new Database.LeadConvert();
instruction.setDoNotCreateOpportunity(true);
System.assertEquals(true,instruction.isDoNotCreateOpportunity());
System.assertEquals(null,ConversionDraftFactory.makeOpportunity(instruction));
instruction.setDoNotCreateOpportunity(false);
System.assertEquals(false,instruction.isDoNotCreateOpportunity());
System.assertNotEquals(null,ConversionDraftFactory.makeOpportunity(instruction));
instruction.setOverwriteLeadSource(true);
System.assertEquals(true,instruction.isOverwriteLeadSource());
instruction.setOverwriteLeadSource(false);
System.assertEquals(false,instruction.isOverwriteLeadSource());
instruction.setSendNotificationEmail(true);
System.assertEquals(true,instruction.isSendNotificationEmail());
instruction.setSendNotificationEmail(false);
System.assertEquals(false,instruction.isSendNotificationEmail());
Database.DMLOptions options = new Database.DMLOptions();
System.assertEquals(null,options.OptAllOrNone);
System.assertEquals(null,options.AllowFieldTruncation);
`},
			} {
				t.Run(api+"/"+route+"/"+probe.name, func(t *testing.T) {
					machine := New(nil)
					if testContext {
						machine.EnableTestContext()
					}
					body, err := CompileAnonymousWithOptions(`return instruction?.isDoNotCreateOpportunity() == false ? new Opportunity(Name='Draft') : null;`, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if err := machine.RegisterClass(Class{Name: "ConversionDraftFactory", APIVersion: api, Methods: map[string]Method{
						"makeOpportunity": {Name: "ConversionDraftFactory.makeOpportunity", ClassName: "ConversionDraftFactory", IsStatic: true, ReturnType: "Opportunity", Params: []Param{{Name: "instruction", Type: "Database.LeadConvert"}}, Program: body, APIVersion: api},
					}}); err != nil {
						t.Fatal(err)
					}
					program, err := CompileAnonymousWithOptions(probe.source, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if _, err := machine.Execute(program); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}
