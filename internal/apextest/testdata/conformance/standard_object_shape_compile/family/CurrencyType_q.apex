List<SObject> l = [SELECT Id FROM CurrencyType LIMIT 1]; System.debug('OBS|CurrencyType_q|' + l.size());
