@IsTest(SeeAllData=false) private class NullProbe62_00 {
 @IsTest static void m00(){ System.assert(false,'CAPTURE|Asset|'+NullCore62.probeObject('Asset',new List<String>{'AccountId','AssetProvidedById','AssetServicedById','ContactId','LocationId'})); }
 @IsTest static void m01(){ System.assert(false,'CAPTURE|AssetWarranty|'+NullCore62.probeObject('AssetWarranty',new List<String>{'Pricebook2Id','WarrantyTermId'})); }
 @IsTest static void m02(){ System.assert(false,'CAPTURE|User|'+NullCore62.probeObject('User',new List<String>{'CallCenterId','ContactId','DelegatedApproverId','IndividualId','ManagerId'})); }
 @IsTest static void m03(){ System.assert(false,'CAPTURE|User|'+NullCore62.probeObject('User',new List<String>{'UserRoleId'})); }
}
