@IsTest private class CheckboxCaptureTest62 {
 @IsTest static void probe() {
  Holder__c offParent=new Holder__c(Name='off',Flag__c=false);
  Holder__c onParent=new Holder__c(Name='on',Flag__c=true);
  insert as system new List<Holder__c>{offParent,onParent};
  List<Item__c> items=new List<Item__c>{new Item__c(Name='missing'),new Item__c(Name='false',Holder__c=offParent.Id),new Item__c(Name='true',Holder__c=onParent.Id)};
  insert as system items;
  for (Item__c item:items) item.Note__c='updated';
  update as system items;
  for (Item__c row:[SELECT Name, IsHeld__c, TextFormula__c FROM Item__c WITH SYSTEM_MODE ORDER BY Name]) CheckboxObserver.save('query',row);
  System.assertEquals(false, ((Map<String,Object>)CheckboxObserver.values.get('insert:missing')).get('flag'), 'insert:missing:flag');
  System.assertEquals(null, ((Map<String,Object>)CheckboxObserver.values.get('insert:missing')).get('text'), 'insert:missing:text');
  System.assertEquals(false, ((Map<String,Object>)CheckboxObserver.values.get('insert:false')).get('flag'), 'insert:false:flag');
  System.assertEquals('off', ((Map<String,Object>)CheckboxObserver.values.get('insert:false')).get('text'), 'insert:false:text');
  System.assertEquals(true, ((Map<String,Object>)CheckboxObserver.values.get('insert:true')).get('flag'), 'insert:true:flag');
  System.assertEquals('on', ((Map<String,Object>)CheckboxObserver.values.get('insert:true')).get('text'), 'insert:true:text');
  System.assertEquals(false, ((Map<String,Object>)CheckboxObserver.values.get('update:missing')).get('flag'), 'update:missing:flag');
  System.assertEquals(null, ((Map<String,Object>)CheckboxObserver.values.get('update:missing')).get('text'), 'update:missing:text');
  System.assertEquals(false, ((Map<String,Object>)CheckboxObserver.values.get('update:false')).get('flag'), 'update:false:flag');
  System.assertEquals('off', ((Map<String,Object>)CheckboxObserver.values.get('update:false')).get('text'), 'update:false:text');
  System.assertEquals(true, ((Map<String,Object>)CheckboxObserver.values.get('update:true')).get('flag'), 'update:true:flag');
  System.assertEquals('on', ((Map<String,Object>)CheckboxObserver.values.get('update:true')).get('text'), 'update:true:text');
  System.assertEquals(false, ((Map<String,Object>)CheckboxObserver.values.get('query:missing')).get('flag'), 'query:missing:flag');
  System.assertEquals(null, ((Map<String,Object>)CheckboxObserver.values.get('query:missing')).get('text'), 'query:missing:text');
  System.assertEquals(false, ((Map<String,Object>)CheckboxObserver.values.get('query:false')).get('flag'), 'query:false:flag');
  System.assertEquals('off', ((Map<String,Object>)CheckboxObserver.values.get('query:false')).get('text'), 'query:false:text');
  System.assertEquals(true, ((Map<String,Object>)CheckboxObserver.values.get('query:true')).get('flag'), 'query:true:flag');
  System.assertEquals('on', ((Map<String,Object>)CheckboxObserver.values.get('query:true')).get('text'), 'query:true:text');
 }
}
