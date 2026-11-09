Map<String, Schema.SObjectType> allTypes = Schema.getGlobalDescribe();
for (String entry : new List<String>{'Visit.PlannedVisitStartTime','AssessmentTask.TaskType','BusinessLicenseApplication.Category','AccountContactRelation.Roles','PricebookEntry.ProductSellingModelId','User.MiddleName'}) {
 List<String> parts = entry.split('\\.');
 if (!allTypes.containsKey(parts[0])) { System.debug('R1ROW|' + entry + '|object_absent'); continue; }
 Map<String, Schema.SObjectField> fields = allTypes.get(parts[0]).getDescribe().fields.getMap();
 if (!fields.containsKey(parts[1])) { System.debug('R1ROW|' + entry + '|field_absent'); continue; }
 Schema.DescribeFieldResult f = fields.get(parts[1]).getDescribe();
 System.debug('R1ROW|' + entry + '|' + f.getType() + '|' + f.isNillable() + '|' + f.isDefaultedOnCreate());
}
