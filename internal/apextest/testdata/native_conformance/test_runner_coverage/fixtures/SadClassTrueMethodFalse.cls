@IsTest(SeeAllData=true)
private class SadClassTrueMethodFalse {
    @IsTest(SeeAllData=false) static void methodFalse() {
        Integer c = [SELECT COUNT() FROM Account WHERE Name = 'ISO_EXISTING_FIXTURE'];
        System.assertEquals(1, c, 'visible-count=' + c);
    }
    @IsTest static void inherits() {
        Integer c = [SELECT COUNT() FROM Account WHERE Name = 'ISO_EXISTING_FIXTURE'];
        System.assertEquals(1, c, 'visible-count=' + c);
        Account a = [SELECT Id, Name FROM Account WHERE Name = 'ISO_EXISTING_FIXTURE' LIMIT 1];
        a.Name = 'ISO_TEMPORARY';
        update a;
    }
}
