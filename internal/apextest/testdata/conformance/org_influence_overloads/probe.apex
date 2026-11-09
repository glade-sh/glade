Map<String,Object> out = new Map<String,Object>();
out.put('qualified', Type.forName('Schema.CampaignInfluence') != null);
out.put('unqualified', Type.forName('CampaignInfluence') != null);
out.put('null_namespace', Type.forName(null, 'Schema.CampaignInfluence') != null);
out.put('empty_namespace', Type.forName('', 'Schema.CampaignInfluence') != null);
out.put('schema_namespace', Type.forName('Schema', 'CampaignInfluence') != null);
out.put('account', Type.forName('Schema.Account') != null);
out.put('absent', Type.forName('Schema.AbsentObject__c') != null);
System.debug('OBSERVED|capture|'+JSON.serialize(out));
