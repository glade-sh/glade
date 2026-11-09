List<SObject> l = [SELECT Id FROM PaymentCredit LIMIT 1]; System.debug('OBS|PaymentCredit_q|' + l.size());
