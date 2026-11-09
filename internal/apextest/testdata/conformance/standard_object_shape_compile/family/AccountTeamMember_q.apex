List<SObject> l = [SELECT Id FROM AccountTeamMember LIMIT 1]; System.debug('OBS|AccountTeamMember_q|' + l.size());
