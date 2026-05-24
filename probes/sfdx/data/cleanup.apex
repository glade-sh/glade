// Clear probe test records before seeding fresh data
List<ProbeTestObject__c> toDelete = [SELECT Id FROM ProbeTestObject__c];
if (!toDelete.isEmpty()) {
    delete toDelete;
}
List<Contact> contacts = [SELECT Id FROM Contact WHERE LastName = 'Probe' AND Account.Name = 'GLADE Probe Account'];
if (!contacts.isEmpty()) {
    delete contacts;
}
List<Account> accounts = [SELECT Id FROM Account WHERE Name = 'GLADE Probe Account'];
if (!accounts.isEmpty()) {
    delete accounts;
}
