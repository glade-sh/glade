@IsTest private class AbsentAddressAssertion {
    @IsTest static void probe() {
        OrgWideEmailAddress found = new OrgWideEmailAddress();
        try {
            found = [SELECT Id, DisplayName FROM OrgWideEmailAddress WHERE DisplayName = 'synthetic-unconfigured-sender' WITH SECURITY_ENFORCED LIMIT 1];
        } catch (Exception e) {}
        System.assertNotEquals(null, found.Id, 'missing address ID');
    }
}
