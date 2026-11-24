OrgWideEmailAddress found = new OrgWideEmailAddress();
try {
    found = [SELECT Id, DisplayName FROM OrgWideEmailAddress WHERE DisplayName = 'synthetic-unconfigured-sender' WITH SECURITY_ENFORCED LIMIT 1];
} catch (Exception e) {}
System.assertEquals(null, found.Id, 'missing address');
