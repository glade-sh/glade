for (String objectName : new List<String>{'Visit','AssessmentTask','BusinessLicenseApplication','AccountContactRelation','PricebookEntry','User'}) {
 Schema.SObjectType st = Schema.getGlobalDescribe().get(objectName);
 Map<String, Schema.SObjectField> f = st.getDescribe().fields.getMap();
 System.debug('R1BOUND|' + objectName + '|' + f.containsKey('Id') + '|' + f.containsKey('GladeMissing__c'));
}
Visit v = new Visit(PlannedVisitStartTime = Datetime.newInstance(2025, 1, 2, 3, 4, 5));
System.debug('R1BEHAVIOR|VisitDatetime|' + (v.PlannedVisitStartTime != null));
try { Database.query('SELECT Id FROM AccountContactRelation WHERE Roles INCLUDES (\'x\')'); System.debug('R1BEHAVIOR|RolesIncludes|accepted'); } catch (Exception e) { System.debug('R1BEHAVIOR|RolesIncludes|' + e.getTypeName()); }
for (String objectName : new List<String>{'AssessmentTask','BusinessLicenseApplication'}) {
 SObject row = Schema.getGlobalDescribe().get(objectName).newSObject();
 Database.SaveResult r = Database.insert(row, false);
 for (Database.Error e : r.getErrors()) { System.debug('R1BEHAVIOR|' + objectName + 'Missing|' + e.getStatusCode() + '|' + String.join(e.getFields(), ',')); }
}
