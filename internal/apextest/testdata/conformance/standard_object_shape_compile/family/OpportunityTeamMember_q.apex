List<SObject> l = [SELECT Id FROM OpportunityTeamMember LIMIT 1]; System.debug('OBS|OpportunityTeamMember_q|' + l.size());
