@IsTest(SeeAllData=false) private class NullProbe67_00 {
 @IsTest static void m00(){ System.assert(false,'CAPTURE|ContentVersion|'+NullCore67.probeObject('ContentVersion',new List<String>{'ContentBodyId','ExternalDataSourceId','FirstPublishLocationId'})); }
 @IsTest static void m01(){ System.assert(false,'CAPTURE|EmailMessage|'+NullCore67.probeObject('EmailMessage',new List<String>{'ActivityId'})); }
 @IsTest static void m02(){ System.assert(false,'CAPTURE|FeedAttachment|'+NullCore67.probeObject('FeedAttachment',new List<String>{'RecordId'})); }
}
