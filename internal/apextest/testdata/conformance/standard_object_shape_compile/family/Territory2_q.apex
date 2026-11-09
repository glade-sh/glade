List<SObject> l = [SELECT Id FROM Territory2 LIMIT 1]; System.debug('OBS|Territory2_q|' + l.size());
