@isTest
private class PermMuteOther {
    private static Map<String,Object> exceptionData(Exception ex){
        Map<String,Object> v=new Map<String,Object>{'typeName'=>ex.getTypeName(),'message'=>ex.getMessage(),'stackTrace'=>ex.getStackTraceString()};
        if(ex instanceof DmlException){DmlException dx=(DmlException)ex;List<Object> details=new List<Object>();
            for(Integer i=0;i<dx.getNumDml();i++)details.add(new Map<String,Object>{'ordinal'=>i,'getDmlIndex'=>dx.getDmlIndex(i),'getDmlId'=>dx.getDmlId(i),'getDmlMessage'=>dx.getDmlMessage(i),'getDmlType'=>dx.getDmlType(i),'getDmlFieldNames'=>dx.getDmlFieldNames(i)});
            v.put('getNumDml',dx.getNumDml());v.put('rows',details);
        }return v;
    }
    private static Map<String,Object> field(Schema.DescribeFieldResult d){return new Map<String,Object>{'accessible'=>d.isAccessible(),'createable'=>d.isCreateable(),'updateable'=>d.isUpdateable(),'permissionable'=>d.isPermissionable(),'nillable'=>d.isNillable()};}
    // Same probes as the native capture, run as a restricted user holding only the group.
    private static void observe(String currentCase,String groupName,Boolean direct,String probeId){
        Profile minimum=[SELECT Id FROM Profile WHERE Name='Minimum Access - Salesforce' AND UserLicense.Name='Salesforce' WITH SYSTEM_MODE];
        PermissionSetGroup groupRow=[SELECT Id FROM PermissionSetGroup WHERE DeveloperName=:groupName WITH SYSTEM_MODE];
        User actor=new User(ProfileId=minimum.Id,Alias='muteU',LastName='Synthetic Muting',Username='mute-observe@example.invalid',Email='mute@example.invalid',TimeZoneSidKey='GMT',LocaleSidKey='en_US',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US',UserRoleId=null);
        User admin=[SELECT Id FROM User WHERE Id=:UserInfo.getUserId() WITH SYSTEM_MODE];
        System.runAs(admin){
            insert as system actor;
            if(direct){
                PermissionSet grant=[SELECT Id FROM PermissionSet WHERE Name='MuteDirect' AND IsOwnedByProfile=false WITH SYSTEM_MODE];
                insert as system new PermissionSetAssignment(AssigneeId=actor.Id,PermissionSetId=grant.Id);
            }
            insert as system new PermissionSetAssignment(AssigneeId=actor.Id,PermissionSetGroupId=groupRow.Id);
        }
        List<SynthMute__c> records=new List<SynthMute__c>{new SynthMute__c(Name='Synthetic own',OwnerId=actor.Id,Other__c='synthetic-own'),new SynthMute__c(Name='Synthetic other',OwnerId=admin.Id,Other__c='synthetic-other')};
        insert as system records;
        Id ownRecord=records[0].Id;Id otherRecord=records[1].Id;
        Map<String,Object> v=new Map<String,Object>{'caseId'=>currentCase};
        System.runAs(actor){
            Id uid=UserInfo.getUserId();
            if(probeId=='actor'){
                Schema.DescribeSObjectResult d=SynthMute__c.SObjectType.getDescribe();
                v.putAll(new Map<String,Object>{'kind'=>'actor','actor'=>'restricted','userId'=>uid,'profileId'=>UserInfo.getProfileId(),'isTest'=>Test.isRunningTest(),'ownRecordId'=>ownRecord,'otherRecordId'=>otherRecord,'objectDescribe'=>new Map<String,Object>{'accessible'=>d.isAccessible(),'createable'=>d.isCreateable(),'updateable'=>d.isUpdateable(),'deletable'=>d.isDeletable()},'fieldDescribe'=>field(SynthMute__c.Other__c.getDescribe())});
            }else{
                v.putAll(new Map<String,Object>{'kind'=>'probe','probeId'=>probeId,'userId'=>uid});
            }
            if(probeId=='select.user'){
                v.put('recordId',ownRecord);
                try{List<SynthMute__c> found=[SELECT Id,Other__c FROM SynthMute__c WHERE Id=:ownRecord WITH USER_MODE];v.put('outcome','success');v.put('records',found);}catch(Exception ex){v.put('outcome','exception');v.put('exception',exceptionData(ex));}
            }else if(probeId=='update.user'){
                v.put('recordId',ownRecord);
                try{SynthMute__c changed=new SynthMute__c(Id=ownRecord,Other__c='synthetic-updated');update as user changed;v.put('outcome','success');v.put('recordId',changed.Id);}catch(Exception ex){v.put('outcome','exception');v.put('exception',exceptionData(ex));}
            }else if(probeId.startsWith('strip.')){
                System.AccessType access=probeId=='strip.readable'?System.AccessType.READABLE:System.AccessType.UPDATABLE;
                List<SObject> input=new List<SObject>{new SynthMute__c(Id=ownRecord,Other__c='synthetic-strip')};
                v.putAll(new Map<String,Object>{'input'=>input,'enforceRootObjectCRUD'=>true});
                try{SObjectAccessDecision decision=Security.stripInaccessible(access,input,true);List<Object> maps=new List<Object>();for(SObject r:decision.getRecords())maps.add(r.getPopulatedFieldsAsMap());v.put('outcome','success');v.put('getRemovedFields',decision.getRemovedFields());v.put('records',decision.getRecords());v.put('recordMaps',maps);}catch(Exception ex){v.put('outcome','exception');v.put('exception',exceptionData(ex));}
            }else if(probeId=='recordAccess'){
                Set<Id> ids=new Set<Id>{ownRecord,otherRecord};
                v.putAll(new Map<String,Object>{'queriedUserId'=>uid,'queryMode'=>'SYSTEM_MODE','recordIds'=>ids});
                try{List<UserRecordAccess> found=[SELECT RecordId,HasReadAccess,HasEditAccess,HasDeleteAccess,MaxAccessLevel FROM UserRecordAccess WHERE UserId=:uid AND RecordId IN :ids WITH SYSTEM_MODE];v.put('outcome','success');v.put('records',found);}catch(Exception ex){v.put('outcome','exception');v.put('exception',exceptionData(ex));}
            }
        }
        // The native capture carries each row through an intentional assertion failure.
        System.assert(false,'OBS|'+JSON.serialize(v));
    }
    @isTest static void M01_FieldEdit_actor(){observe('M01_FieldEdit','M01_FieldEdit',false,'actor');}
    @isTest static void M01_FieldEdit_select_user(){observe('M01_FieldEdit','M01_FieldEdit',false,'select.user');}
    @isTest static void M01_FieldEdit_update_user(){observe('M01_FieldEdit','M01_FieldEdit',false,'update.user');}
    @isTest static void M01_FieldEdit_strip_readable(){observe('M01_FieldEdit','M01_FieldEdit',false,'strip.readable');}
    @isTest static void M01_FieldEdit_strip_updatable(){observe('M01_FieldEdit','M01_FieldEdit',false,'strip.updatable');}
    @isTest static void M01_FieldEdit_recordAccess(){observe('M01_FieldEdit','M01_FieldEdit',false,'recordAccess');}
    @isTest static void F01_GroupOnly_actor(){observe('F01_GroupOnly','F01_FieldReadEdit',false,'actor');}
    @isTest static void F01_GroupOnly_select_user(){observe('F01_GroupOnly','F01_FieldReadEdit',false,'select.user');}
    @isTest static void F01_GroupOnly_update_user(){observe('F01_GroupOnly','F01_FieldReadEdit',false,'update.user');}
    @isTest static void F01_GroupOnly_strip_readable(){observe('F01_GroupOnly','F01_FieldReadEdit',false,'strip.readable');}
    @isTest static void F01_GroupOnly_strip_updatable(){observe('F01_GroupOnly','F01_FieldReadEdit',false,'strip.updatable');}
    @isTest static void F01_GroupOnly_recordAccess(){observe('F01_GroupOnly','F01_FieldReadEdit',false,'recordAccess');}
    @isTest static void F01_WithDirect_actor(){observe('F01_WithDirect','F01_FieldReadEdit',true,'actor');}
    @isTest static void F01_WithDirect_select_user(){observe('F01_WithDirect','F01_FieldReadEdit',true,'select.user');}
    @isTest static void F01_WithDirect_update_user(){observe('F01_WithDirect','F01_FieldReadEdit',true,'update.user');}
    @isTest static void F01_WithDirect_strip_readable(){observe('F01_WithDirect','F01_FieldReadEdit',true,'strip.readable');}
    @isTest static void F01_WithDirect_strip_updatable(){observe('F01_WithDirect','F01_FieldReadEdit',true,'strip.updatable');}
    @isTest static void F01_WithDirect_recordAccess(){observe('F01_WithDirect','F01_FieldReadEdit',true,'recordAccess');}
}
