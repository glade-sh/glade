List<SObject> l = [SELECT Id FROM WebStore LIMIT 1]; System.debug('OBS|WebStore_q|' + l.size());
