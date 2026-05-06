List<ProbeTestObject__c> existing = [SELECT Id FROM ProbeTestObject__c];
if (!existing.isEmpty()) {
    delete existing;
}

List<ProbeTestObject__c> records = new List<ProbeTestObject__c>();
for (Integer i = 1; i <= 3; i++) {
    ProbeTestObject__c record = new ProbeTestObject__c();
    record.Name__c = 'Record' + i;
    record.Value__c = i * 10;
    record.Triggered__c = false;
    record.CurrencyIsoCode = 'USD';
    records.add(record);
}
insert records;
