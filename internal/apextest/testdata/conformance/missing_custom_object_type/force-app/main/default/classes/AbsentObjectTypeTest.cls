@IsTest
private class AbsentObjectTypeTest {
    @IsTest static void typeIsMissing() {
        AbsentBuoy__c recordValue;
        System.assertEquals(null, recordValue);
    }
}
