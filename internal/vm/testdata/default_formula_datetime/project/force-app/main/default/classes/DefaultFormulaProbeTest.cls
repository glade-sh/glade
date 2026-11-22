@IsTest private class DefaultFormulaProbeTest {
 @IsTest static void loaded() { System.assertEquals('CAPTURE', DefaultFormulaProbe.inspect(true, false)); }
 @IsTest static void unloaded() { System.assertEquals('CAPTURE', DefaultFormulaProbe.inspect(false, false)); }
 @IsTest static void inserted() { System.assertEquals('CAPTURE', DefaultFormulaProbe.inspect(false, true)); }
}