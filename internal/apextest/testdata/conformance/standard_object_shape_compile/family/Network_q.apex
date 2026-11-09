List<SObject> l = [SELECT Id FROM Network LIMIT 1]; System.debug('OBS|Network_q|' + l.size());
