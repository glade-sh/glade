List<String> values = new List<String>();
for (Schema.FieldSetMember member : Schema.SObjectType.Thing__c.FieldSets.Shown.getFields()) {
 values.add(member.getFieldPath() + '|' + member.getSObjectField().getDescribe().getName() + '|' + member.getLabel());
}
System.assert(false, 'P|F001|' + String.join(values, ';'));
