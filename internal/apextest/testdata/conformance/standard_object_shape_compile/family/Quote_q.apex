List<SObject> l = [SELECT Id FROM Quote LIMIT 1]; System.debug('OBS|Quote_q|' + l.size());
