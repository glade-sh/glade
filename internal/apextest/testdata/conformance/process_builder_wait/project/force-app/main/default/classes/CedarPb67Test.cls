@IsTest private class CedarPb67Test {
    @IsTest static void process() { System.assertEquals('SENTINEL', CedarPb67.process()); }
    @IsTest static void waits() { System.assertEquals('SENTINEL', CedarPb67.waits()); }
}
