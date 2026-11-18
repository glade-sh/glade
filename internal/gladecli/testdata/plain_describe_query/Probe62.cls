@IsTest
private class Probe62 {
 @IsTest static void runProbe() {
List<String> results = new List<String>();
List<String> names = new List<String>{'Time_Dependency__c', 'FSL__Time_Dependency__c', 'fsl__time_dependency__c'};
for (String n : names) { results.add('RESULT|initial|' + n + '|' + Schema.getGlobalDescribe().containsKey(n) + '|' + (Schema.getGlobalDescribe().get(n) != null)); }
Map<String, Schema.SObjectType> warmed = Schema.getGlobalDescribe();
for (String n : names) { results.add('RESULT|warm|' + n + '|' + Schema.getGlobalDescribe().containsKey(n) + '|' + (Schema.getGlobalDescribe().get(n) != null)); }
Schema.SObjectType standardType = Account.SObjectType;
results.add('RESULT|control|Account|' + (standardType != null));
for (String n : names) { results.add('RESULT|standard|' + n + '|' + Schema.getGlobalDescribe().containsKey(n) + '|' + (Schema.getGlobalDescribe().get(n) != null)); }
results.add('RESULT|size|' + Schema.getGlobalDescribe().size());
for (String n : new List<String>{'Time_Dependency__c', 'FSL__Time_Dependency__c'}) {
 results.add('RESULT|type|' + n + '|' + (Type.forName(n) != null));
 try { results.add('RESULT|describe|' + n + '|' + Schema.describeSObjects(new List<String>{n}).size()); } catch (Exception e) { results.add('RESULT|describe|' + n + '|EXCEPTION|' + e.getMessage()); }
 try { results.add('RESULT|query|' + n + '|' + Database.query('SELECT Id FROM ' + n + ' LIMIT 1').size()); } catch (Exception e) { results.add('RESULT|query|' + n + '|EXCEPTION|' + e.getMessage()); }
}

System.debug('BEGIN_RESULTS');
for (String item : results) System.debug(item);
System.debug('END_RESULTS');
System.assert(false, 'BEGIN_RESULTS~' + String.join(results, '~') + '~END_RESULTS');
 }
}
