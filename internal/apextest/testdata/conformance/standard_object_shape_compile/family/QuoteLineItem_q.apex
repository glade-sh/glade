List<SObject> l = [SELECT Id FROM QuoteLineItem LIMIT 1]; System.debug('OBS|QuoteLineItem_q|' + l.size());
