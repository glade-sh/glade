List<SObject> l = [SELECT Id FROM PickTicket LIMIT 1]; System.debug('OBS|PickTicket_q|' + l.size());
