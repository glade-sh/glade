@IsTest(SeeAllData=false) private class NullProbe67_00 {
 @IsTest static void m00(){ System.assert(false,'CAPTURE|ContractLineItem|'+NullCore67.probeObject('ContractLineItem',new List<String>{'AssetId','LocationId','ParentContractLineItemId'})); }
 @IsTest static void m01(){ System.assert(false,'CAPTURE|OpportunityLineItem|'+NullCore67.probeObject('OpportunityLineItem',new List<String>{'PricebookEntryId','Product2Id'})); }
 @IsTest static void m02(){ System.assert(false,'CAPTURE|ProductConsumed|'+NullCore67.probeObject('ProductConsumed',new List<String>{'PricebookEntryId','ProductItemId','WorkOrderLineItemId'})); }
}
