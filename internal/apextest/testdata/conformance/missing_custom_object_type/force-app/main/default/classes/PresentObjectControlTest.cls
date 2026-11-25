@IsTest
private class PresentObjectControlTest {
    @IsTest static void presentTypesCompile() {
        PresentBuoy__c customRecord = new PresentBuoy__c(Name='Control');
        Account standardRecord = new Account(Name='Control');
        System.assertNotEquals(null, customRecord);
        System.assertNotEquals(null, standardRecord);
    }
}
