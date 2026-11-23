ProbeFlag__mdt value = ProbeFlag__mdt.getInstance('Case');
System.assertNotEquals(null, value); System.assertEquals(true, value.Active__c);
System.assertEquals(null, ProbeFlag__mdt.getInstance('Missing'));
System.debug('METADATA=pass');
