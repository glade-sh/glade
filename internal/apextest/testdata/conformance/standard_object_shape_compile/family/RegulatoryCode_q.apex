List<SObject> l = [SELECT Id FROM RegulatoryCode LIMIT 1]; System.debug('OBS|RegulatoryCode_q|' + l.size());
