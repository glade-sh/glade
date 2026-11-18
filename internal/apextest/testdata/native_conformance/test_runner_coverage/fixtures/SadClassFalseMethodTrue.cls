@IsTest(SeeAllData=false)
private class SadClassFalseMethodTrue {
    @IsTest(SeeAllData=true) static void methodTrue() {
        Integer c = [SELECT COUNT() FROM Account WHERE Name = 'ISO_EXISTING_FIXTURE'];
        System.assertEquals(1, c, 'visible-count=' + c);
    }
    @IsTest static void plain() {
        Integer c = [SELECT COUNT() FROM Account WHERE Name = 'ISO_EXISTING_FIXTURE'];
        System.assertEquals(0, c, 'visible-count=' + c);
    }
}
