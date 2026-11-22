@IsTest private class MetadataTriggerTest { @IsTest static void run() {
ProbeFlag__mdt config = ProbeFlag__mdt.getInstance('Case');
System.assertNotEquals(null, config); System.assertEquals(true, config.Active__c); System.assertEquals('Case', config.ObjectName__c);
System.assertEquals(null, ProbeFlag__mdt.getInstance('Missing'));
Case record = new Case(Subject='Synthetic'); insert record;
System.debug('TRIGGER=pass');

} }
