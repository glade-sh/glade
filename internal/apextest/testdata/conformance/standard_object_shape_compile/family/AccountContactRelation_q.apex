List<SObject> l = [SELECT Id FROM AccountContactRelation LIMIT 1]; System.debug('OBS|AccountContactRelation_q|' + l.size());
