@IsTest
private class SadDefault {
    @IsTest static void t() {
        Integer c = [SELECT COUNT() FROM Account WHERE Name = 'ISO_EXISTING_FIXTURE'];
        System.assertEquals(0, c, 'visible-count=' + c);
        System.assert([SELECT COUNT() FROM User] > 0, 'users visible');
        System.assert([SELECT COUNT() FROM Profile] > 0, 'profiles visible');
    }
}
