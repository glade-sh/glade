Map<String,Object> result = new Map<String,Object>();
for (String name : new List<String>{'Account','Knowledge__ka','Knowledge__kav','AbsentObject__c'}) {
    result.put(name+'.describe', Schema.getGlobalDescribe().containsKey(name));
    result.put(name+'.type', Type.forName('Schema.'+name) != null);
    try { Database.countQuery('SELECT COUNT() FROM '+name); result.put(name+'.query', true); }
    catch (Exception e) { result.put(name+'.query', e.getTypeName()); }
}
System.debug('OBSERVED|capture|'+JSON.serialize(result));
