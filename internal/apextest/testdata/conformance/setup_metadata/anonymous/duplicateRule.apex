List<DuplicateRule> records = [SELECT DeveloperName, MasterLabel, IsActive, SobjectType FROM DuplicateRule WHERE DeveloperName='Grove_Email_Check'];
        Map<String,Object> observed = new Map<String,Object>{'count'=>records.size()};
        if (!records.isEmpty()) {
            DuplicateRule row=records[0]; observed.put('name',row.DeveloperName); observed.put('label',row.MasterLabel); observed.put('active',row.IsActive); observed.put('object',row.SobjectType);
        }
        System.assert(false,'OBSERVED|duplicateRule|'+JSON.serialize(observed));
