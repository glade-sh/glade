Map<String,Object> observed = new Map<String,Object>();
for (ChildRow67__mdt row : [SELECT DeveloperName, Parent__c, Parent__r.DeveloperName, Parent__r.Label FROM ChildRow67__mdt ORDER BY DeveloperName]) {
 observed.put(row.DeveloperName, new Map<String,Object>{'parent'=>row.Parent__r.DeveloperName, 'label'=>row.Parent__r.Label, 'linked'=>row.Parent__c!=null});
}
observed.put('label_count', [SELECT COUNT() FROM ChildRow67__mdt WHERE Parent__r.Label = 'Long parent']);
System.assert(false, 'METADATA_BATCH|' + JSON.serialize(observed));
