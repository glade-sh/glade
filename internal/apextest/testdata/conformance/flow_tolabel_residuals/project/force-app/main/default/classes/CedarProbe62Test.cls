@IsTest private class CedarProbe62Test {
    @IsTest static void flows() { System.assertEquals('SENTINEL', CedarProbe62.run()); }
    @IsTest static void grouped() { System.assertEquals('SENTINEL', CedarProbe62.groupedRun()); }
}
