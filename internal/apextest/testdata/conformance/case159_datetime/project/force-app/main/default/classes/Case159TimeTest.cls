@IsTest private class Case159TimeTest {
 @IsTest static void storedPrecision() {
  Datetime a=Datetime.newInstance(1777723200007L);
  RoadTime__c row=new RoadTime__c(Name='Synthetic',Moment__c=a); insert row;
  RoadTime__c back=[SELECT Moment__c FROM RoadTime__c WHERE Id=:row.Id];
  System.assertEquals(1777723200007L,row.Moment__c.getTime());
  System.assertEquals(1777723200000L,back.Moment__c.getTime());
  row.Moment__c=Datetime.newInstance(1777723200999L); update row;
  back=[SELECT Moment__c FROM RoadTime__c WHERE Id=:row.Id];
  System.assertEquals(1777723200999L,row.Moment__c.getTime());
  System.assertEquals(1777723200000L,back.Moment__c.getTime());
 }
}
