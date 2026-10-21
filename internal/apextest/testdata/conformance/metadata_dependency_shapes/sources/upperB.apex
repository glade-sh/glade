Set<String> found = new Set<String>();
        for (MeadowSouth__mdt sample : [SELECT Code__c FROM MeadowSouth__mdt WHERE Code__c != null]) {
            found.add(sample.Code__c.toUpperCase());
        }
        MeadowSouth__mdt seed = new MeadowSouth__mdt(Code__c='ca');
System.debug('NATIVE|' + (seed.Code__c.toUpperCase() + ':' + found.size()));
