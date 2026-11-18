List<List<SObject>> groups = Search.query('FIND {unrelated} RETURNING Gl_Obj__c(Id ORDER BY Open__c) WITH USER_MODE');
System.assert(false, 'OBS|groups=' + groups.size() + ',count=' + groups[0].size());
