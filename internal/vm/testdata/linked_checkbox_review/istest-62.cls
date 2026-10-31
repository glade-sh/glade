@IsTest private class Road291Test62 {
 @IsTest static void observe() {
  Parent__c p=new Parent__c(Name='parent',Flag__c=null,FalseFlag__c=null);
  Child__c child=new Child__c(Parent__r=p);
  System.assertEquals(false,child.Parent__r.Flag__c); System.assertEquals(null,child.Parent__r.get('Flag__c'));
  p.put('Flag__c',null); System.assertEquals(false,child.Parent__r.Flag__c); System.assertEquals(null,child.Parent__r.get('Flag__c'));
  System.assertEquals(false,child.Parent__r.FalseFlag__c); System.assertEquals(null,child.Parent__r.get('FalseFlag__c'));
  Parent__c saved=new Parent__c(Name='saved'); insert saved;
  System.assertEquals(false,saved.Flag__c); System.assertEquals(null,saved.get('Flag__c'));
  Parent__c idOnly=new Parent__c(Id=saved.Id); System.assertEquals(false,idOnly.Flag__c); System.assertEquals(null,idOnly.get('Flag__c'));
  Parent__c q=[SELECT Flag__c FROM Parent__c WHERE Id=:saved.Id]; System.assertEquals(true,q.Flag__c); System.assertEquals(true,q.get('Flag__c'));
  Boolean rejected=false; try {insert p;} catch(DmlException e) {rejected=true; System.assertEquals('INVALID_TYPE_ON_FIELD_IN_RECORD',String.valueOf(e.getDmlStatusCode(0)));} System.assert(rejected);
  SObject w=Schema.getGlobalDescribe().get('WebStore').newSObject(); w.put('OptionsAutoFacetingEnabled',null); System.assertEquals(null,w.get('OptionsAutoFacetingEnabled'));
 }
}