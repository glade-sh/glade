Set<String> found = new Set<String>();
        for (MeadowNorth__mdt sample : [SELECT Code__c FROM MeadowNorth__mdt WHERE Code__c != null]) {
            found.add(sample.Code__c.toUpperCase());
        }
        MeadowNorth__mdt seed = new MeadowNorth__mdt(Code__c='ca');
System.debug('NATIVE|' + (seed.Code__c.toUpperCase() + ':' + found.size()));
