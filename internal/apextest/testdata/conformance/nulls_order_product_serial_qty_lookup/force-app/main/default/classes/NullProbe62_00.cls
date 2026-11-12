@IsTest(SeeAllData=false) private class NullProbe62_00 {
 @IsTest static void m00(){ System.assert(false,'CAPTURE|ProductConsumed|'+NullCore62.probeObject('ProductConsumed',new List<String>{'ProductItemId'})); }
 @IsTest static void m01(){ System.assert(false,'CAPTURE|SerializedProduct|'+NullCore62.probeObject('SerializedProduct',new List<String>{'ProductItemId'})); }
}
