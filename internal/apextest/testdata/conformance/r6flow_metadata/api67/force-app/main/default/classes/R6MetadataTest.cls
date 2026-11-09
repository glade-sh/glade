@IsTest private class R6MetadataTest {
 @IsTest static void found() { System.assertEquals('MATCH', R6MetadataProbe.observe('matched')); }
 @IsTest static void absent() { System.assertEquals('PASS', R6MetadataProbe.observe('absent')); }
}
