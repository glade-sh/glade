List<SObject> l = [SELECT Id FROM Territory2Model LIMIT 1]; System.debug('OBS|Territory2Model_q|' + l.size());
