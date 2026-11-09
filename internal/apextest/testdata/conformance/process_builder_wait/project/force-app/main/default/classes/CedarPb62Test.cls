@IsTest private class CedarPb62Test {
    @IsTest static void process() { System.assertEquals('SENTINEL', CedarPb62.process()); }
    @IsTest static void waits() { System.assertEquals('SENTINEL', CedarPb62.waits()); }
}
