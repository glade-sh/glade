@IsTest private class OpportunityDescribeObservation {
    @IsTest static void picklist() {
        List<String> values = new List<String>();
        for (Schema.PicklistEntry entry : Opportunity.Type.getDescribe().getPicklistValues()) {
            values.add(entry.getValue());
        }
        String observed = String.join(values, '|');
        System.assert(false, 'OBSERVED|picklist|' + observed + '|END');
    }
    private static String permissions() {
        Schema.DescribeSObjectResult described = Opportunity.SObjectType.getDescribe();
        return String.valueOf(described.isAccessible()) + ',' + described.isCreateable() + ',' +
            described.isUpdateable() + ',' + described.isDeletable() + ',' + described.isUndeletable();
    }
    @IsTest static void chatter() {
        String admin = permissions();
        Profile profile = [SELECT Id FROM Profile WHERE Name = 'Chatter Free User' LIMIT 1];
        User actor = new User(ProfileId=profile.Id, Username='parcel.' + System.now().getTime() + '@example.invalid',
            Email='parcel@example.invalid', LastName='Parcel', Alias='parcel',
            TimeZoneSidKey='America/Los_Angeles', LocaleSidKey='en_US',
            EmailEncodingKey='UTF-8', LanguageLocaleKey='en_US');
        insert actor;
        String restricted;
        System.runAs(actor) { restricted = permissions(); }
        String observed = admin + '|' + restricted;
        System.assert(false, 'OBSERVED|chatter|' + observed + '|END');
    }
}
