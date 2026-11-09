List<SObject> l = [SELECT Id FROM NetworkMember LIMIT 1]; System.debug('OBS|NetworkMember_q|' + l.size());
