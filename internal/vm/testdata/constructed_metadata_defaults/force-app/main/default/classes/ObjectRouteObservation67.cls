public class ObjectRouteObservation67 { public static void observe() {
List<String> routes=new List<String>{'new','json-typed','json-generic','json-map-dynamic','newSObject','newSObject-false','newSObject-true','clone','clone-explicit'};
for(String route:routes) {
 DefaultThing__c row;
 if(route=='new') row=new DefaultThing__c();
 else if(route=='json-typed') row=(DefaultThing__c)JSON.deserialize('{}',DefaultThing__c.class);
 else if(route=='json-generic') row=(DefaultThing__c)JSON.deserialize('{"attributes":{"type":"DefaultThing__c"}}',SObject.class);
 else if(route=='json-map-dynamic') {
  Map<String,Object> fields=new Map<String,Object>{'Name'=>'Product'};
  row=(DefaultThing__c)JSON.deserialize(JSON.serialize(fields),Type.forName('Schema.DefaultThing__c'));
 }
 else if(route=='newSObject') row=(DefaultThing__c)DefaultThing__c.SObjectType.newSObject();
 else if(route=='newSObject-false') row=(DefaultThing__c)DefaultThing__c.SObjectType.newSObject(null,false);
 else if(route=='newSObject-true') row=(DefaultThing__c)DefaultThing__c.SObjectType.newSObject(null,true);
 else if(route=='clone') row=new DefaultThing__c().clone(false,true,false,false);
 else row=new DefaultThing__c(Priority__c=7,Enabled__c=true,Title__c='set',Choice__c='Second').clone(false,true,false,false);
 System.debug('ROUTE|'+route+'|Priority__c|dot|'+String.valueOf(row.Priority__c));
 System.debug('ROUTE|'+route+'|Enabled__c|dot|'+String.valueOf(row.Enabled__c));
 System.debug('ROUTE|'+route+'|Title__c|dot|'+String.valueOf(row.Title__c));
 System.debug('ROUTE|'+route+'|Choice__c|dot|'+String.valueOf(row.Choice__c));
 for(String field:new List<String>{'Priority__c','Enabled__c','Title__c','Choice__c'}) {
  System.debug('ROUTE|'+route+'|'+field+'|get|'+String.valueOf(row.get(field)));
  System.debug('ROUTE|'+route+'|'+field+'|populated|'+String.valueOf(row.getPopulatedFieldsAsMap().containsKey(field)));
 }
 Metadata.CustomMetadata dto=new Metadata.CustomMetadata();
 dto.fullName=row.getSObjectType().getDescribe().getName().removeEnd('__c')+'.Product';
 System.debug('ROUTE|'+route+'|fullName|dto|'+dto.fullName);
}
} }
