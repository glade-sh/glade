@IsTest private class SchemaNativeTest {
 @IsTest static void observe() {
  Map<String,Schema.SObjectType> allTypes = Schema.getGlobalDescribe();
  for (String value : new List<String>{'Visit.PlannedVisitStartTime|DATETIME|false|false','AssessmentTask.TaskType|PICKLIST|false|false','BusinessLicenseApplication.Category|PICKLIST|false|false','AccountContactRelation.Roles|MULTIPICKLIST|true|false','PricebookEntry.ProductSellingModelId|REFERENCE|true|false','User.MiddleName|STRING|true|false'}) {
   List<String> row=value.split('\\|'); List<String> names=row[0].split('\\.');
   Map<String,Schema.SObjectField> fields=allTypes.get(names[0]).getDescribe().fields.getMap();
   System.assert(fields.containsKey('Id')); System.assert(!fields.containsKey('GladeMissing__c'));
   Schema.DescribeFieldResult f=fields.get(names[1]).getDescribe();
   System.assertEquals(row[1],String.valueOf(f.getType()).toUpperCase());
   System.assertEquals(row[2],String.valueOf(f.isNillable()));
   System.assertEquals(row[3],String.valueOf(f.isDefaultedOnCreate()));
  }
  Visit v=new Visit(PlannedVisitStartTime=Datetime.newInstance(2025,1,2,3,4,5)); System.assertNotEquals(null,v.PlannedVisitStartTime);
  List<SObject> found=Database.query('SELECT Id FROM AccountContactRelation WHERE Roles INCLUDES (\'x\')'); System.assertEquals(0,found.size());
  for (String name : new List<String>{'AssessmentTask','BusinessLicenseApplication'}) {
   Database.SaveResult r=Database.insert(allTypes.get(name).newSObject(),false);
   System.assert(!r.isSuccess()); System.assertEquals(StatusCode.REQUIRED_FIELD_MISSING,r.getErrors()[0].getStatusCode());
   System.assert(r.getErrors()[0].getFields().contains(name=='AssessmentTask'?'TaskType':'Category'));
  }
 }
}
