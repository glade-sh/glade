@IsTest private class CedarProbe67Test {
    @IsTest static void flows() { System.assertEquals('SENTINEL', CedarProbe67.run()); }
    @IsTest static void grouped() { System.assertEquals('SENTINEL', CedarProbe67.groupedRun()); }
}
