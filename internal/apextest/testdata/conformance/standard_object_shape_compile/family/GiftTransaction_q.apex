List<SObject> l = [SELECT Id FROM GiftTransaction LIMIT 1]; System.debug('OBS|GiftTransaction_q|' + l.size());
