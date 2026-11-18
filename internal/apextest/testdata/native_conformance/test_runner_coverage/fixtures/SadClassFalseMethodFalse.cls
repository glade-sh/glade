@IsTest(SeeAllData=false)
private class SadClassFalseMethodFalse {
    @IsTest(SeeAllData=false) static void t() {
        Integer c = [SELECT COUNT() FROM Account WHERE Name = 'ISO_EXISTING_FIXTURE'];
        System.assertEquals(0, c, 'visible-count=' + c);
    }
}
