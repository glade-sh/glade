List<SObject> l = [SELECT Id FROM OpportunitySplit LIMIT 1]; System.debug('OBS|OpportunitySplit_q|' + l.size());
