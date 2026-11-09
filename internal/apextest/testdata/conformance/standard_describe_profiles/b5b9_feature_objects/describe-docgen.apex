Map<String,Schema.SObjectType> allTypes=Schema.getGlobalDescribe();
for(String n:new List<String>{'DocumentTemplateContentDoc','DocumentGenerationProcess'}) {
 Schema.SObjectType typ=allTypes.get(n);if(typ==null) {System.debug('CAPTURE '+n+' null');continue;}
 Schema.DescribeSObjectResult obj=typ.getDescribe(); Map<String,Schema.SObjectField> f=obj.fields.getMap(); List<String> names=new List<String>(f.keySet());names.sort();
 List<Object> fields=new List<Object>();
 for(String k:names) {Schema.DescribeFieldResult d=f.get(k).getDescribe();List<String> refs=new List<String>();for(Schema.SObjectType ref:d.getReferenceTo())refs.add(String.valueOf(ref));
 fields.add(new Map<String,Object>{'name'=>d.getName(),'type'=>String.valueOf(d.getType()),'label'=>d.getLabel(),'length'=>d.getLength(),'precision'=>d.getPrecision(),'scale'=>d.getScale(),'nillable'=>d.isNillable(),'createable'=>d.isCreateable(),'updateable'=>d.isUpdateable(),'calculated'=>d.isCalculated(),'defaultedOnCreate'=>d.isDefaultedOnCreate(),'relationshipName'=>d.getRelationshipName(),'referenceTo'=>refs});}
 System.debug('CAPTURE '+n+' '+JSON.serialize(new Map<String,Object>{'name'=>obj.getName(),'label'=>obj.getLabel(),'keyPrefix'=>obj.getKeyPrefix(),'queryable'=>obj.isQueryable(),'createable'=>obj.isCreateable(),'updateable'=>obj.isUpdateable(),'fields'=>fields}));
}
