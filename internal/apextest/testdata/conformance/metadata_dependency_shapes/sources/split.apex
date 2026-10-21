MeadowWords__mdt sample = new MeadowWords__mdt(Words__c='oak,pine');
        List<String> pieces = sample.Words__c.split(',');
System.debug('NATIVE|' + (String.join(pieces, ':') + ':' + MeadowWords__mdt.getAll().size()));
