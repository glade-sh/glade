List<SObject> l = [SELECT Id FROM Survey LIMIT 1]; System.debug('OBS|Survey_q|' + l.size());
