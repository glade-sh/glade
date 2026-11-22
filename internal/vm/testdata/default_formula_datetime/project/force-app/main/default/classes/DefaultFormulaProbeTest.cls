@IsTest private class DefaultFormulaProbeTest {
 @IsTest static void rtLoaded() { System.assertEquals('CAPTURE', DefaultFormulaProbe.inspect('rtLoaded')); }
 @IsTest static void rtUnloaded() { System.assertEquals('CAPTURE', DefaultFormulaProbe.inspect('rtUnloaded')); }
 @IsTest static void nullLoaded() { System.assertEquals('CAPTURE', DefaultFormulaProbe.inspect('nullLoaded')); }
 @IsTest static void rtInsert() { System.assertEquals('CAPTURE', DefaultFormulaProbe.inspect('rtInsert')); }
 @IsTest static void plainInsert() { System.assertEquals('CAPTURE', DefaultFormulaProbe.inspect('plainInsert')); }
}
