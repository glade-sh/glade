@IsTest private class ShorthandProbe67 {
 @IsTest static void run() {
 Schema.FieldSet one = SObjectType.Probe__c.FieldSets.MainList;
 Schema.FieldSet two = Schema.SObjectType.Probe__c.FieldSets.Main;
 System.assertEquals(1, one.getFields().size());
 System.assertEquals(1, two.getFields().size());
 }
}
