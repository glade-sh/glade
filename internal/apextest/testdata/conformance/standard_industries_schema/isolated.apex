Map<String, Schema.SObjectType> globalTypes = Schema.getGlobalDescribe();
for (String pair : new List<String>{'AccountContactRelation.Roles','PricebookEntry.ProductSellingModelId','User.MiddleName'}) {
 List<String> p = pair.split('\\.');
 if (!globalTypes.containsKey(p[0])) { System.debug('R1ISO|' + pair + '|object_absent'); continue; }
 Map<String, Schema.SObjectField> f = globalTypes.get(p[0]).getDescribe().fields.getMap();
 if (!f.containsKey(p[1])) { System.debug('R1ISO|' + pair + '|field_absent'); continue; }
 Schema.DescribeFieldResult x = f.get(p[1]).getDescribe();
 System.debug('R1ISO|' + pair + '|' + x.getType() + '|' + x.isNillable() + '|' + x.isDefaultedOnCreate());
}
