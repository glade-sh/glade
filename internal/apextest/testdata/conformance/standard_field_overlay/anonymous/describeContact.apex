Map<String,Object> result = new Map<String,Object>();
Schema.DescribeFieldResult d10 = Contact.SObjectType.getDescribe().fields.getMap().get('CleanStatus').getDescribe();
List<String> refs10=new List<String>(); for(Schema.SObjectType target:d10.getReferenceTo()) refs10.add(String.valueOf(target));
List<String> picks10=new List<String>(); for(Schema.PicklistEntry entry:d10.getPicklistValues()) picks10.add(entry.getValue()+':'+entry.isActive()+':'+entry.isDefaultValue());
result.put('Contact.CleanStatus',new Map<String,Object>{'type'=>String.valueOf(d10.getType()),'length'=>d10.getLength(),'precision'=>d10.getPrecision(),'scale'=>d10.getScale(),'nillable'=>d10.isNillable(),'createable'=>d10.isCreateable(),'updateable'=>d10.isUpdateable(),'permissionable'=>d10.isPermissionable(),'defaultedOnCreate'=>d10.isDefaultedOnCreate(),'referenceTo'=>refs10,'relationshipName'=>d10.getRelationshipName(),'picklist'=>picks10});
System.assert(false,'FIELD_OVERLAY|describeContact|'+JSON.serialize(result));
