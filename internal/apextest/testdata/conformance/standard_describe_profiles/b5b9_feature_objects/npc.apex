Map<String,Schema.SObjectType> allTypes=Schema.getGlobalDescribe();
for(String n:new List<String>{'GiftTransaction','GiftCommitment','DocumentTemplateContentDoc','DocumentGenerationProcess','RegulatoryCode'}) {
 Schema.SObjectType typ=allTypes.get(n);if(typ==null) {System.debug('ROW '+n+' object-absent');continue;}
 Schema.DescribeSObjectResult d=typ.getDescribe(); Map<String,Schema.SObjectField> m=d.fields.getMap();
 System.debug('ROW '+n+' fields '+JSON.serialize(new List<String>(m.keySet())));
}
