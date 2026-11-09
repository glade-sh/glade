List<SObject> l = [SELECT Id FROM DocumentTemplate LIMIT 1]; System.debug('OBS|DocumentTemplate_q|' + l.size());
