String kind = 'boundary';
Map<String,Object> observation = new Map<String,Object>();
if (kind != 'boundary' && kind != 'rejection') {
    Schema.SObjectField direct = Opportunity.Type;
    String objectName = 'Opportunity'; String fieldName = 'Type';
    if (kind == 'lead') { direct = Lead.LeadSource; objectName = 'Lead'; fieldName = 'LeadSource'; }
    if (kind == 'account') { direct = Account.Industry; objectName = 'Account'; fieldName = 'Industry'; }
    List<Schema.SObjectField> tokens = new List<Schema.SObjectField>{
        direct, Schema.getGlobalDescribe().get(objectName).getDescribe().fields.getMap().get(fieldName),
        Schema.describeSObjects(new List<String>{objectName})[0].fields.getMap().get(fieldName)
    };
    List<Boolean> routesMatch = new List<Boolean>();
    String first;
    for (Schema.SObjectField token : tokens) {
        Schema.DescribeFieldResult field = token.getDescribe();
        List<Object> entries = new List<Object>();
        for (Schema.PicklistEntry entry : field.getPicklistValues()) {
            entries.add(new List<Object>{entry.getValue(),entry.getLabel(),entry.isActive(),entry.isDefaultValue()});
        }
        String serialized = JSON.serialize(entries);
        if (first == null) { first = serialized; observation.put('entries',entries); }
        routesMatch.add(first == serialized);
        observation.put('type',String.valueOf(field.getType()));
        observation.put('restricted',field.isRestrictedPicklist());
    }
    observation.put('routesMatch',routesMatch);
} else if (kind == 'boundary') {
    observation.put('nameValues',Account.Name.getDescribe().getPicklistValues().size());
    observation.put('nameRestricted',Account.Name.getDescribe().isRestrictedPicklist());
} else if (kind == 'rejection') {
    Schema.SObjectField missing = Account.SObjectType.getDescribe().fields.getMap().get('MissingChoice__c');
    observation.put('missing',missing == null);
    try { missing.getDescribe(); observation.put('error','accepted'); }
    catch (Exception e) { observation.put('error',e.getTypeName()); }
}
System.assert(false,'PICKLIST|boundary|'+JSON.serialize(observation));
