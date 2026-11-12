@IsTest(SeeAllData=false) private class NullProbe62_00 {
 @IsTest static void m00(){ System.assert(false,'CAPTURE|ContentVersion|'+NullCore62.probeObject('ContentVersion',new List<String>{'ContentBodyId','ExternalDataSourceId','FirstPublishLocationId'})); }
 @IsTest static void m01(){ System.assert(false,'CAPTURE|EmailMessage|'+NullCore62.probeObject('EmailMessage',new List<String>{'ActivityId'})); }
 @IsTest static void m02(){ System.assert(false,'CAPTURE|FeedAttachment|'+NullCore62.probeObject('FeedAttachment',new List<String>{'RecordId'})); }
}
