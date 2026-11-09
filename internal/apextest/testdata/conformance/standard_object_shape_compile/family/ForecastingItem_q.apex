List<SObject> l = [SELECT Id FROM ForecastingItem LIMIT 1]; System.debug('OBS|ForecastingItem_q|' + l.size());
