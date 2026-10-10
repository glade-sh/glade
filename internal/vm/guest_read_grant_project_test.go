package vm_test

import (
	"encoding/json"
	"testing"
)

// SF213 exact API62/project40 read-only grant and field boundary assertion.
func TestSitesGuestReadOnlyGrantSF213(t *testing.T) {
	report := runGuestFLSProject(t, map[string]string{"GladeGuestReadGrant62Assertions.cls": `@IsTest private class GladeGuestReadGrant62Assertions {
 private static Map<String,Object> flags() {
  Schema.DescribeSObjectResult obj=Contact.SObjectType.getDescribe();
  Map<String,Object> result=new Map<String,Object>{'objectRead'=>obj.isAccessible(),'objectCreate'=>obj.isCreateable(),'objectUpdate'=>obj.isUpdateable()};
  for(String name:new List<String>{'LastName','Description'}) {
   Schema.DescribeFieldResult field=obj.fields.getMap().get(name).getDescribe();
   result.put(name,new Map<String,Object>{'read'=>field.isAccessible(),'create'=>field.isCreateable(),'update'=>field.isUpdateable()});
  }
  return result;
 }
 @IsTest static void assertReadOnlyObjectAndFieldGrantBoundaries() {
  Set<String> names=new Set<String>{'Standard Guest','Guest License User'};
  List<Profile> profiles=[SELECT Id FROM Profile WHERE Name IN :names];
  System.assert(!profiles.isEmpty(),'SF200 Sites prerequisite');
  Profile chosen=[SELECT Name,UserType FROM Profile WHERE Id=:profiles[0].Id];
  Map<String,Object> result=new Map<String,Object>{'profileName'=>chosen.Name,'profileUserType'=>chosen.UserType};
  User guest=new User(Alias='jdoe',Email='owned@example.invalid',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US',LastName='Doe',LocaleSidKey='en_US',ProfileId=chosen.Id,TimeZoneSidKey='America/Los_Angeles',Username='gladeguestreadgrant62assert.'+UserInfo.getOrganizationId()+'.'+Datetime.now().getTime()+'@example.invalid');
  System.runAs(guest){result.put('before',flags());}
  try {
   PermissionSet ps=new PermissionSet(Name='GladeGuestReadGrant62AssertOwned',Label='Owned guest grant');insert ps;
   result.put('permissionSetCreated',true);
   ObjectPermissions permission=new ObjectPermissions(ParentId=ps.Id,SobjectType='Contact',PermissionsRead=true,PermissionsCreate=false,PermissionsEdit=false,PermissionsDelete=false,PermissionsViewAllRecords=false,PermissionsModifyAllRecords=false);
   insert permission;result.put('objectPermissionCreated',true);
   try {
    insert new PermissionSetAssignment(AssigneeId=guest.Id,PermissionSetId=ps.Id);
    result.put('assignmentCreated',true);
   }catch(Exception error){result.put('assignmentErrorType',error.getTypeName());result.put('assignmentErrorMessage',error.getMessage());}
   System.runAs(guest){result.put('objectOnly',flags());}
   for(String fieldName:new List<String>{'LastName','Description'}) {
    try {
     insert new FieldPermissions(ParentId=ps.Id,SobjectType='Contact',Field='Contact.'+fieldName,PermissionsRead=true,PermissionsEdit=true);
     result.put(fieldName+'GrantCreated',true);
    }catch(Exception error){result.put(fieldName+'GrantErrorType',error.getTypeName());result.put(fieldName+'GrantErrorMessage',error.getMessage());}
    System.runAs(guest){result.put('after'+fieldName+'GrantAttempt',flags());}
   }
  }catch(Exception error){result.put('setupErrorType',error.getTypeName());result.put('setupErrorMessage',error.getMessage());}
  System.assertEquals(true,result.get('permissionSetCreated'),'permissionSetCreated');
  System.assertEquals(true,result.get('objectPermissionCreated'),'objectPermissionCreated');
  System.assertEquals(true,result.get('assignmentCreated'),'assignmentCreated');
  System.assertEquals(true,result.get('DescriptionGrantCreated'),'DescriptionGrantCreated');
  System.assertEquals('Guest',result.get('profileUserType'));
  System.assertEquals(false,result.containsKey('setupErrorType'));
  System.assertEquals(false,result.containsKey('assignmentErrorType'));
  System.assertEquals(false,result.containsKey('LastNameGrantCreated'));
  System.assertEquals(false,result.containsKey('DescriptionGrantErrorType'));
  System.assertEquals('System.DmlException',result.get('LastNameGrantErrorType'));
  System.assertEquals('Insert failed. First exception on row 0; first error: INVALID_OR_NULL_FOR_RESTRICTED_PICKLIST, Field Name: bad value for restricted picklist field: Contact.LastName: [Field]',result.get('LastNameGrantErrorMessage'));
  { Map<String,Object> actual=(Map<String,Object>)result.get('before');
   System.assertEquals(false,actual.get('objectRead'),'before.objectRead');
   System.assertEquals(false,actual.get('objectCreate'),'before.objectCreate');
   System.assertEquals(false,actual.get('objectUpdate'),'before.objectUpdate');
   { Map<String,Object> field=(Map<String,Object>)actual.get('LastName');
    System.assertEquals(false,field.get('read'),'before.LastName.read');
    System.assertEquals(false,field.get('create'),'before.LastName.create');
    System.assertEquals(false,field.get('update'),'before.LastName.update');
   }
   { Map<String,Object> field=(Map<String,Object>)actual.get('Description');
    System.assertEquals(false,field.get('read'),'before.Description.read');
    System.assertEquals(false,field.get('create'),'before.Description.create');
    System.assertEquals(false,field.get('update'),'before.Description.update');
   }
  }
  { Map<String,Object> actual=(Map<String,Object>)result.get('objectOnly');
   System.assertEquals(true,actual.get('objectRead'),'objectOnly.objectRead');
   System.assertEquals(false,actual.get('objectCreate'),'objectOnly.objectCreate');
   System.assertEquals(false,actual.get('objectUpdate'),'objectOnly.objectUpdate');
   { Map<String,Object> field=(Map<String,Object>)actual.get('LastName');
    System.assertEquals(true,field.get('read'),'objectOnly.LastName.read');
    System.assertEquals(false,field.get('create'),'objectOnly.LastName.create');
    System.assertEquals(false,field.get('update'),'objectOnly.LastName.update');
   }
   { Map<String,Object> field=(Map<String,Object>)actual.get('Description');
    System.assertEquals(true,field.get('read'),'objectOnly.Description.read');
    System.assertEquals(false,field.get('create'),'objectOnly.Description.create');
    System.assertEquals(false,field.get('update'),'objectOnly.Description.update');
   }
  }
  { Map<String,Object> actual=(Map<String,Object>)result.get('afterLastNameGrantAttempt');
   System.assertEquals(true,actual.get('objectRead'),'afterLastNameGrantAttempt.objectRead');
   System.assertEquals(false,actual.get('objectCreate'),'afterLastNameGrantAttempt.objectCreate');
   System.assertEquals(false,actual.get('objectUpdate'),'afterLastNameGrantAttempt.objectUpdate');
   { Map<String,Object> field=(Map<String,Object>)actual.get('LastName');
    System.assertEquals(true,field.get('read'),'afterLastNameGrantAttempt.LastName.read');
    System.assertEquals(false,field.get('create'),'afterLastNameGrantAttempt.LastName.create');
    System.assertEquals(false,field.get('update'),'afterLastNameGrantAttempt.LastName.update');
   }
   { Map<String,Object> field=(Map<String,Object>)actual.get('Description');
    System.assertEquals(true,field.get('read'),'afterLastNameGrantAttempt.Description.read');
    System.assertEquals(false,field.get('create'),'afterLastNameGrantAttempt.Description.create');
    System.assertEquals(false,field.get('update'),'afterLastNameGrantAttempt.Description.update');
   }
  }
  { Map<String,Object> actual=(Map<String,Object>)result.get('afterDescriptionGrantAttempt');
   System.assertEquals(true,actual.get('objectRead'),'afterDescriptionGrantAttempt.objectRead');
   System.assertEquals(false,actual.get('objectCreate'),'afterDescriptionGrantAttempt.objectCreate');
   System.assertEquals(false,actual.get('objectUpdate'),'afterDescriptionGrantAttempt.objectUpdate');
   { Map<String,Object> field=(Map<String,Object>)actual.get('LastName');
    System.assertEquals(true,field.get('read'),'afterDescriptionGrantAttempt.LastName.read');
    System.assertEquals(false,field.get('create'),'afterDescriptionGrantAttempt.LastName.create');
    System.assertEquals(false,field.get('update'),'afterDescriptionGrantAttempt.LastName.update');
   }
   { Map<String,Object> field=(Map<String,Object>)actual.get('Description');
    System.assertEquals(true,field.get('read'),'afterDescriptionGrantAttempt.Description.read');
    System.assertEquals(false,field.get('create'),'afterDescriptionGrantAttempt.Description.create');
    System.assertEquals(false,field.get('update'),'afterDescriptionGrantAttempt.Description.update');
   }
  }
 }
}
`})
	if got := report.Summary(); got.Total != 1 || got.Passed != 1 || got.Errors != 0 {
		logGuestFLSProblems(t, report)
		detail, _ := json.Marshal(report)
		t.Fatalf("SF213: %+v %s", got, detail)
	}
}
